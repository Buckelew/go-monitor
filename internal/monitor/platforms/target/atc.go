package target

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	http "github.com/bogdanfinn/fhttp"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
	"github.com/buckelew/go-monitor/internal/solver"
)

const (
	atcEndpoint    = "https://carts.target.com/web_checkouts/v1/cart_items?field_groups=CART%2CCART_ITEMS%2CSUMMARY&key=feaf228eb2777fd3eee0fd5192ae7107d6224b39"
	bypassEndpoint = "https://carts.target.com/web_checkouts/v1/multiple_cart_items?field_groups=CART%2CCART_ITEMS%2CSUMMARY&key=feaf228eb2777fd3eee0fd5192ae7107d6224b39"
)

type ATCTask struct {
	id          int32
	tcin        string
	url         string
	delay       int32
	isEnabled   bool
	client      *httpclient.Client
	solver      *solver.Client
	queries     *database.Queries
	lastSuccess bool
}

func NewATCTask(queries database.Queries, client *httpclient.Client, solver *solver.Client, tcin string, task database.Task) *ATCTask {
	return &ATCTask{
		id:        task.ID,
		tcin:      tcin,
		url:       fmt.Sprintf("https://www.target.com/p/-/A-%s", tcin),
		delay:     task.Delay,
		isEnabled: task.Enabled,
		client:    client,
		solver:    solver,
		queries:   &queries,
	}
}

func (t *ATCTask) ID() int32              { return t.id }
func (t *ATCTask) Platform() monitor.Platform { return monitor.Target }
func (t *ATCTask) Type() monitor.TaskType  { return monitor.ATC }
func (t *ATCTask) Delay() int32            { return t.delay }
func (t *ATCTask) IsEnabled() bool         { return t.isEnabled }

func (t *ATCTask) Run(ctx context.Context) (*monitor.TaskResult, error) {
	if err := t.client.RotateProxy(); err != nil {
		return nil, fmt.Errorf("failed to rotate proxy: %w", err)
	}

	// TODO: convert proxy URL to solver format
	shapeHeaders, err := t.solver.Solve(ctx, "", "target:atc")
	if err != nil {
		return nil, fmt.Errorf("solver failed: %w", err)
	}

	// Try the primary ATC endpoint
	success, err := t.tryATC(ctx, shapeHeaders)
	if err != nil {
		return nil, err
	}

	// If primary failed with 424/forbidden, try the bypass endpoint
	if !success {
		log.Printf("[task %d] primary ATC failed, trying bypass endpoint", t.id)
		success, err = t.tryBypass(ctx, shapeHeaders)
		if err != nil {
			return nil, err
		}
	}

	events := t.handleResult(ctx, success)

	return &monitor.TaskResult{
		Success: success,
		Events:  events,
	}, nil
}

func (t *ATCTask) tryATC(ctx context.Context, shapeHeaders map[string]string) (bool, error) {
	body := map[string]any{
		"cart_type":        "REGULAR",
		"channel_id":       "90",
		"shopping_context": "DIGITAL",
		"cart_item": map[string]any{
			"tcin":            t.tcin,
			"quantity":        1,
			"item_channel_id": "10",
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("failed to marshal ATC body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, atcEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return false, fmt.Errorf("failed to create ATC request: %w", err)
	}

	setATCHeaders(req, shapeHeaders)

	resp, err := t.client.Inner().Do(req)
	if err != nil {
		return false, fmt.Errorf("ATC request failed: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // drain body

	log.Printf("[task %d] ATC response status: %d", t.id, resp.StatusCode)

	return resp.StatusCode == 200, nil
}

func (t *ATCTask) tryBypass(ctx context.Context, shapeHeaders map[string]string) (bool, error) {
	cartItem := map[string]any{
		"cart_item": map[string]any{
			"tcin":            t.tcin,
			"quantity":        1,
			"item_channel_id": "10",
		},
	}

	body := map[string]any{
		"cart_type":  "REGULAR",
		"cart_items": []any{cartItem},
		"target":     []any{cartItem},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("failed to marshal bypass body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, bypassEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return false, fmt.Errorf("failed to create bypass request: %w", err)
	}

	setATCHeaders(req, shapeHeaders)

	resp, err := t.client.Inner().Do(req)
	if err != nil {
		return false, fmt.Errorf("bypass request failed: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // drain body

	log.Printf("[task %d] bypass response status: %d", t.id, resp.StatusCode)

	return resp.StatusCode == 200, nil
}

func (t *ATCTask) handleResult(ctx context.Context, success bool) []monitor.ItemEvent {
	var events []monitor.ItemEvent

	if success && !t.lastSuccess {
		log.Printf("[task %d] ATC success for TCIN %s", t.id, t.tcin)
		t.lastSuccess = true

		item, err := t.queries.UpsertItem(ctx, database.UpsertItemParams{
			TaskID:   t.id,
			Url:      t.url,
			Platform: string(monitor.Target),
			Data:     json.RawMessage(`{}`),
			InStock:  true,
		})
		if err != nil {
			log.Printf("[task %d] failed to upsert item: %v", t.id, err)
			return events
		}

		t.insertEvent(ctx, item.ID, json.RawMessage(`{"in_stock": false}`), json.RawMessage(`{"in_stock": true}`))

		events = append(events, monitor.ItemEvent{
			Type: monitor.EventRestock,
			Item: monitor.Item{
				URL:     t.url,
				Title:   fmt.Sprintf("Target TCIN %s", t.tcin),
				InStock: true,
			},
			ItemID: item.ID,
		})
	}

	if !success && t.lastSuccess {
		log.Printf("[task %d] ATC failed for TCIN %s (was previously successful)", t.id, t.tcin)
		t.lastSuccess = false

		existing, err := t.queries.GetItemByTaskAndURL(ctx, database.GetItemByTaskAndURLParams{
			TaskID: t.id,
			Url:    t.url,
		})
		if err != nil {
			log.Printf("[task %d] failed to look up item for OOS update: %v", t.id, err)
			return events
		}

		if err := t.queries.UpdateItemInStock(ctx, database.UpdateItemInStockParams{
			ID:      existing.ID,
			InStock: false,
		}); err != nil {
			log.Printf("[task %d] failed to update item in_stock: %v", t.id, err)
		}
	}

	return events
}

func (t *ATCTask) insertEvent(ctx context.Context, itemID int32, prevState, newState json.RawMessage) {
	_, err := t.queries.InsertItemEvent(ctx, database.InsertItemEventParams{
		ItemID:        itemID,
		PreviousState: prevState,
		NewState:      newState,
	})
	if err != nil {
		log.Printf("[task %d] failed to insert item event: %v", t.id, err)
	}
}

func setATCHeaders(req *http.Request, shapeHeaders map[string]string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Origin", "https://www.target.com")
	req.Header.Set("Referer", "https://www.target.com/")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Application-Name", "web")
	req.Header.Set("Sec-Ch-Ua", `"Not(A:Brand";v="99", "Google Chrome";v="133", "Chromium";v="133"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Dnt", "1")

	for key, value := range shapeHeaders {
		req.Header.Set(key, value)
	}
}

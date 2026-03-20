package shopify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/shopify"
)

// TestFetchProducts_PaginationBodyLeak verifies that response bodies are closed
// between paginated requests rather than deferred until function return.
//
// When defer resp.Body.Close() is inside a for loop, all response bodies stay
// open until the function returns. This prevents connection reuse: the client
// opens a new TCP connection for each page instead of reusing one.
func TestFetchProducts_PaginationBodyLeak(t *testing.T) {
	const numPages = 5

	var activeConns, maxConns atomic.Int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageStr)
		if page == 0 {
			page = 1
		}

		count := 250
		if page >= numPages {
			count = 10
		}

		products := make([]json.RawMessage, count)
		for i := range products {
			products[i] = json.RawMessage(fmt.Sprintf(
				`{"title":"P%d-%d","handle":"p%d-%d","variants":[{"available":true}],"images":[]}`,
				page, i, page, i))
		}
		json.NewEncoder(w).Encode(map[string]any{"products": products})
	})

	srv := httptest.NewUnstartedServer(handler)
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			cur := activeConns.Add(1)
			for {
				old := maxConns.Load()
				if cur <= old || maxConns.CompareAndSwap(old, cur) {
					break
				}
			}
		case http.StateClosed:
			activeConns.Add(-1)
		}
	}
	srv.Start()
	defer srv.Close()

	client, err := httpclient.New(nil)
	require.NoError(t, err)

	scraper := shopify.NewSearchShopify(srv.URL, client)

	result, err := scraper.FetchProducts(context.Background())
	require.NoError(t, err)

	expectedItems := (numPages-1)*250 + 10
	assert.Len(t, result.Items, expectedItems, "should fetch all items across %d pages", numPages)

	// With proper body closure between pages, the client reuses a single
	// connection. With defer-in-loop, each page opens a new connection
	// because the previous response body was never closed.
	max := maxConns.Load()
	t.Logf("max concurrent connections: %d (pages: %d)", max, numPages)
	assert.LessOrEqual(t, max, int32(2),
		"opened %d connections for %d pages; bodies not closed between pages", max, numPages)
}

package solver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultBaseURL = "https://tools.pacificaio.com"

type Client struct {
	baseURL string
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{baseURL: baseURL}
}

type solveRequest struct {
	Proxy string `json:"proxy"`
	Site  string `json:"site"`
}

func (c *Client) Solve(ctx context.Context, proxy string, site string) (map[string]string, error) {
	body, err := json.Marshal(solveRequest{Proxy: proxy, Site: site})
	if err != nil {
		return nil, fmt.Errorf("solver: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/solve", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("solver: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("solver: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("solver: unexpected status %d", resp.StatusCode)
	}

	// TODO: parse solver response once API contract is finalized
	return map[string]string{}, nil
}

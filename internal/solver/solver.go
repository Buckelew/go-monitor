package solver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultBaseURL = "https://tools.pacificaio.com"

type Client struct {
	baseURL  string
	apiToken string
}

func New(baseURL, apiToken string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{baseURL: baseURL, apiToken: apiToken}
}

type solveRequest struct {
	Site  string `json:"site"`
	Proxy string `json:"proxy"`
}

// SolveResult contains the Shape solver response.
type SolveResult struct {
	ShapeHeaders map[string]string `json:"shapeHeaders"`
	Prefix       string            `json:"prefix"`
	Cookies      string            `json:"cookies"`
	Ms           int               `json:"ms"`
}

func (c *Client) Solve(ctx context.Context, proxy string, site string) (*SolveResult, error) {
	body, err := json.Marshal(solveRequest{Site: site, Proxy: proxy})
	if err != nil {
		return nil, fmt.Errorf("solver: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/shape/solve", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("solver: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("solver: post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("solver: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("solver: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result SolveResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("solver: parse response: %w", err)
	}

	return &result, nil
}

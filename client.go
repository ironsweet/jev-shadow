package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Client struct {
	apiBase    string
	timeout    time.Duration
	httpClient *http.Client
}

func NewClient(url string, timeout time.Duration) *Client {
	return &Client{
		apiBase: url,
		timeout: timeout,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 3 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 5 * time.Second,
				MaxIdleConnsPerHost:   10,
			},
		},
	}
}

func (cc *Client) Judge(ctx context.Context, state string, questionId string, instructions string) (*SystemOneResponse, error) {
	var reqBody bytes.Buffer
	err := json.NewEncoder(&reqBody).Encode(SystemOneRequest{
		State: state,
		Questions: map[string]Question{
			questionId: {
				Type:         "noul",
				Instructions: instructions,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cc.apiBase+"/v1/systemone", &reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var systemResp SystemOneResponse
	err = json.NewDecoder(resp.Body).Decode(&systemResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &systemResp, nil
}

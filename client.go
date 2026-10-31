package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiBase    string
	httpClient *http.Client
}

func NewClient(url string, timeout time.Duration) *Client {
	return &Client{
		apiBase: strings.TrimRight(url, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 3 * time.Second,
				}).DialContext,
				MaxIdleConnsPerHost: 10,
			},
		},
	}
}

func (cc *Client) Judge(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
	state := fmt.Sprintf("user_intent: %s\ntool_calls: %s", userIntent, toolCalls)
	var reqBody bytes.Buffer
	err := json.NewEncoder(&reqBody).Encode(SystemOneRequest{
		State: state,
		Questions: map[string]Question{
			"intent_covered": {
				Type:         "noul",
				Instructions: "Is provided tool calls enough to implement user intention?",
			},
		},
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cc.apiBase+"/v1/systemone", &reqBody)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 0, 0, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}

	var systemResp SystemOneResponse
	err = json.NewDecoder(resp.Body).Decode(&systemResp)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode response body: %w", err)
	}

	prob, ok := systemResp.NoulProbability("intent_covered")
	if !ok {
		return 0, 0, fmt.Errorf("response missing 'intent_covered' probability")
	}
	return prob, systemResp.LatencyMs, nil
}

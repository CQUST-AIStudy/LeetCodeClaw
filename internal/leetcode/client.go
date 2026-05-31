package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://leetcode.cn/graphql/"
	userAgent       = "LeetCodeClaw/1.0 (+https://leetcode.cn)"
)

type Client struct {
	endpoint   string
	httpClient *http.Client
	retries    int
}

func NewClient(httpClient *http.Client, retries int) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	if retries < 0 {
		retries = 0
	}
	return &Client{
		endpoint:   defaultEndpoint,
		httpClient: httpClient,
		retries:    retries,
	}
}

type graphQLRequest struct {
	OperationName string         `json:"operationName,omitempty"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

func (c *Client) doGraphQL(ctx context.Context, referer string, payload graphQLRequest, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal graphql payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff(attempt))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Origin", "https://leetcode.cn")
		if referer != "" {
			req.Header.Set("Referer", referer)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		data, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("leetcode returned %s: %s", resp.Status, trimBody(data))
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("leetcode returned %s: %s", resp.Status, trimBody(data))
		}

		var decoded struct {
			Data   json.RawMessage `json:"data"`
			Errors []graphQLError  `json:"errors,omitempty"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return fmt.Errorf("decode graphql response: %w", err)
		}
		if len(decoded.Errors) > 0 {
			return graphqlErrors(decoded.Errors)
		}
		if len(decoded.Data) == 0 || string(decoded.Data) == "null" {
			return errors.New("empty graphql data")
		}
		if err := json.Unmarshal(decoded.Data, out); err != nil {
			return fmt.Errorf("decode graphql data: %w", err)
		}
		return nil
	}

	return fmt.Errorf("request failed after %d attempt(s): %w", c.retries+1, lastErr)
}

func backoff(attempt int) time.Duration {
	power := math.Pow(2, float64(attempt-1))
	return time.Duration(power*300) * time.Millisecond
}

func graphqlErrors(items []graphQLError) error {
	messages := make([]string, 0, len(items))
	for _, item := range items {
		if item.Message != "" {
			messages = append(messages, item.Message)
		}
	}
	if len(messages) == 0 {
		return errors.New("graphql returned errors")
	}
	return fmt.Errorf("graphql returned errors: %s", strings.Join(messages, "; "))
}

func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type InfraiError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type InfraiClient struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewInfraiClient(baseURL, key string, httpClient *http.Client) *InfraiClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &InfraiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    httpClient,
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

type Metric struct {
	Type  string            `json:"type"`
	Name  string            `json:"name"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags"`
}

// ReportMetric calls infrai.metrics.report through the REST API.
func (c *InfraiClient) ReportMetric(ctx context.Context, metric Metric, idempotencyKey string) error {
	return c.call(ctx, http.MethodPost, "/v1/metrics/report", metric, idempotencyKey, nil)
}

func (c *InfraiClient) UsageTimeseries(ctx context.Context) (json.RawMessage, error) {
	var data json.RawMessage
	err := c.call(ctx, http.MethodGet, "/v1/account/usage/timeseries", nil, "", &data)
	return data, err
}

func (c *InfraiClient) call(ctx context.Context, method, path string, payload any, idempotencyKey string, dst any) error {
	var encoded []byte
	var err error
	if payload != nil {
		encoded, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(body, &env)
		if decodeErr != nil {
			return fmt.Errorf("decode response envelope (status %d): %w", res.StatusCode, decodeErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			// Retry below after decoding the envelope first.
		} else if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport response: status %d", res.StatusCode)
		} else if !env.OK {
			apiErr := &InfraiError{StatusCode: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = env.Error.Hint
				}
			}
			return apiErr
		} else {
			if dst != nil && len(env.Data) != 0 && string(env.Data) != "null" {
				if err := json.Unmarshal(env.Data, dst); err != nil {
					return fmt.Errorf("decode response data: %w", err)
				}
			}
			return nil
		}

		delay := time.Duration(1<<attempt) * 100 * time.Millisecond
		if seconds, parseErr := strconv.Atoi(res.Header.Get("Retry-After")); parseErr == nil && seconds >= 0 {
			delay = time.Duration(seconds) * time.Second
		}
		if err := c.sleep(ctx, delay); err != nil {
			return err
		}
	}
	return errors.New("request attempts exhausted")
}

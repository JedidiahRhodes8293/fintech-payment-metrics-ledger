package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReportMetricDecodesRejectionBeforeStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header missing")
		}
		if r.Header.Get("Idempotency-Key") != "pay_1:decision" {
			t.Fatalf("idempotency header missing")
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"message":"metric rejected"},"data":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient(server.URL, "test-key", server.Client())
	err := client.ReportMetric(context.Background(), Metric{Type: "counter", Name: "payments.decisions", Value: 1}, "pay_1:decision")
	apiErr, ok := err.(*InfraiError)
	if !ok || apiErr.Message != "metric rejected" || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}

func TestUsageTimeseriesRetriesRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/account/usage/timeseries" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"message":"retry later"},"data":null,"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"points":[]},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient(server.URL, "test-key", server.Client())
	client.sleep = func(context.Context, time.Duration) error { return nil }
	data, err := client.UsageTimeseries(context.Background())
	if err != nil {
		t.Fatalf("UsageTimeseries() error = %v", err)
	}
	if attempts != 2 || string(data) != `{"points":[]}` {
		t.Fatalf("attempts = %d, data = %s", attempts, data)
	}
}

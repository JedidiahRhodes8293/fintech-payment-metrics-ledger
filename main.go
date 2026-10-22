package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	client := NewInfraiClient(baseURL, key, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments", func(w http.ResponseWriter, r *http.Request) {
		var event PaymentEvent
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment event"})
			return
		}
		result, err := ProcessPayment(r.Context(), client, event)
		if err != nil {
			var apiErr *InfraiError
			if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Error()})
				return
			}
			if _, decisionErr := DecidePayment(event); decisionErr != nil {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": decisionErr.Error()})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "metric reporting failed"})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	})
	mux.HandleFunc("GET /platform-usage", func(w http.ResponseWriter, r *http.Request) {
		data, err := client.UsageTimeseries(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "usage query failed"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("payment metric service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

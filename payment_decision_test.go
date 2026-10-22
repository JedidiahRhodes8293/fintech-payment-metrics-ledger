package main

import (
	"context"
	"testing"
)

type recordedReporter struct {
	metrics []Metric
	keys    []string
}

func (r *recordedReporter) ReportMetric(_ context.Context, metric Metric, key string) error {
	r.metrics = append(r.metrics, metric)
	r.keys = append(r.keys, key)
	return nil
}

func TestProcessPaymentRiskDecision(t *testing.T) {
	tests := []struct {
		name      string
		riskScore int
		want      string
		wantAudit string
	}{
		{name: "ordinary payment", riskScore: 22, want: "approve", wantAudit: "PAYMENT_APPROVED"},
		{name: "review boundary", riskScore: 70, want: "review", wantAudit: "RISK_REVIEW"},
		{name: "decline boundary", riskScore: 90, want: "decline", wantAudit: "RISK_DECLINED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &recordedReporter{}
			event := PaymentEvent{
				PaymentID: "pay_2048", MerchantID: "merchant_7", Currency: "USD",
				AmountCents: 12500, RiskScore: tt.riskScore,
			}
			got, err := ProcessPayment(context.Background(), reporter, event)
			if err != nil {
				t.Fatalf("ProcessPayment() error = %v", err)
			}
			if got.Action != tt.want || got.Notification.AuditCode != tt.wantAudit {
				t.Fatalf("decision = %q/%q, want %q/%q", got.Action, got.Notification.AuditCode, tt.want, tt.wantAudit)
			}
			if len(reporter.metrics) != 2 {
				t.Fatalf("reported %d metrics, want 2", len(reporter.metrics))
			}
			if reporter.metrics[0].Tags["decision"] != tt.want {
				t.Fatalf("metric decision tag = %q, want %q", reporter.metrics[0].Tags["decision"], tt.want)
			}
			if reporter.keys[0] != "pay_2048:payments.decisions" {
				t.Fatalf("idempotency key = %q", reporter.keys[0])
			}
		})
	}
}

package main

import (
	"context"
	"fmt"
)

type PaymentEvent struct {
	PaymentID   string `json:"payment_id"`
	MerchantID  string `json:"merchant_id"`
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amount_cents"`
	RiskScore   int    `json:"risk_score"`
}

type Notification struct {
	PaymentID string `json:"payment_id"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason"`
	AuditCode string `json:"audit_code"`
}

type PaymentResult struct {
	Action       string       `json:"action"`
	Notification Notification `json:"notification"`
}

type MetricReporter interface {
	ReportMetric(context.Context, Metric, string) error
}

func DecidePayment(event PaymentEvent) (PaymentResult, error) {
	if event.PaymentID == "" || event.MerchantID == "" || event.Currency == "" || event.AmountCents <= 0 {
		return PaymentResult{}, fmt.Errorf("payment_id, merchant_id, currency, and positive amount_cents are required")
	}

	decision, reason, auditCode := "approve", "risk score below review threshold", "PAYMENT_APPROVED"
	if event.RiskScore >= 90 {
		decision, reason, auditCode = "decline", "risk score reached decline threshold", "RISK_DECLINED"
	} else if event.RiskScore >= 70 {
		decision, reason, auditCode = "review", "risk score reached manual review threshold", "RISK_REVIEW"
	}

	return PaymentResult{
		Action: decision,
		Notification: Notification{
			PaymentID: event.PaymentID,
			Decision:  decision,
			Reason:    reason,
			AuditCode: auditCode,
		},
	}, nil
}

func ProcessPayment(ctx context.Context, reporter MetricReporter, event PaymentEvent) (PaymentResult, error) {
	result, err := DecidePayment(event)
	if err != nil {
		return PaymentResult{}, err
	}
	tags := map[string]string{
		"merchant_id": event.MerchantID,
		"currency":    event.Currency,
		"decision":    result.Action,
	}
	metrics := []Metric{
		{Type: "counter", Name: "payments.decisions", Value: 1, Tags: tags},
		{Type: "gauge", Name: "payments.amount_cents", Value: float64(event.AmountCents), Tags: tags},
	}
	for _, metric := range metrics {
		requestID := event.PaymentID + ":" + metric.Name
		if err := reporter.ReportMetric(ctx, metric, requestID); err != nil {
			return PaymentResult{}, fmt.Errorf("report %s: %w", metric.Name, err)
		}
	}
	return result, nil
}

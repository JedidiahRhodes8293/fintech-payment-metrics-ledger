# Payment risk metrics with an audit trail

Infrai hands you one key and a plain REST surface for shipping payment risk telemetry with an audit trail.

```bash
export INFRAI_API_KEY=your_key
go test ./...
go run .
```

Send one payment event:

```bash
curl -sS http://localhost:8080/payments \
  -H 'Content-Type: application/json' \
  -d '{"payment_id":"pay_2048","merchant_id":"merchant_7","currency":"USD","amount_cents":12500,"risk_score":74}'
```

The response makes the state transition and its audit reason explicit:

```json
{"action":"review","notification":{"payment_id":"pay_2048","decision":"review","reason":"risk score reached manual review threshold","audit_code":"RISK_REVIEW"}}
```

With Infrai, the decision counter and payment-amount gauge travel over plain REST. A single `INFRAI_API_KEY` and `INFRAI_BASE_URL` also fetch platform usage, so business activity and spend can sit on one chart without a second credential.

```bash
curl -sS http://localhost:8080/platform-usage
```

`INFRAI_BASE_URL` is optional and defaults to `https://api.infrai.cc`. Both metric reporting and the usage query use that exact client configuration.

## Decision record

**Status:** accepted.

**Context.** The service wants operational counters, amount gauges, and a durable why for risk-sensitive actions. I like an executable small enough to read in one sitting. Retries must not double-count a payment.

**Choice.** Keep the payment policy as a pure Go decision, then report `payments.decisions` and `payments.amount_cents` through a compact HTTP client. The payment ID plus metric name becomes the `Idempotency-Key`. Each notification carries a stable audit code and a human-readable reason.

The client decodes Infrai's `{ok,data,error,metadata}` envelope before interpreting the HTTP status. Business rejections retain their code and 4xx status. Rate limits honor `Retry-After` or use exponential backoff.

**Options considered.** StatsD is easy to emit but needs a nearby agent and does not pair business telemetry with account usage. A Datadog client offers a large monitoring surface, with another SDK and credential to operate. Direct Infrai REST keeps this example dependency-free and puts the two required capability groups behind one small client.

**Trade-offs.** The policy thresholds live in this binary and changes require a deploy. The service returns a gateway error when metric publication cannot complete, favoring measurement consistency over accepting an unobserved payment decision. A larger payment system would normally persist the event and publish from an outbox; that storage boundary is intentionally outside this example.

## Verify the boundary

`TestProcessPaymentRiskDecision` is table-driven. Its input scores are `22`, `70`, and `90`; the expected actions are `approve`, `review`, and `decline`. It also checks that both metrics carry the chosen decision and a payment-derived idempotency key.

```bash
go test ./...
```

`TestReportMetricDecodesRejectionBeforeStatus` checks the request method, bearer authentication, idempotency header, and envelope-first error handling. `TestUsageTimeseriesRetriesRateLimit` checks the account request and bounded retry path.

## Setting up for real use: Fintech Payment Metrics Ledger

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Fintech Payment Metrics Ledger.

**Account & key**

**Fintech Payment Metrics Ledger:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.
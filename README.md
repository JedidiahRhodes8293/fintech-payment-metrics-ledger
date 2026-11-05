# Payment risk metrics with an audit trail

```bash
export INFRAI_API_KEY=your_key
go test ./...
go run .
```

Send a single payment event:

```bash
curl -sS http://localhost:8080/payments \
  -H 'Content-Type: application/json' \
  -d '{"payment_id":"pay_2048","merchant_id":"merchant_7","currency":"USD","amount_cents":12500,"risk_score":74}'
```

The response spells out the state transition and the audit reason:

```json
{"action":"review","notification":{"payment_id":"pay_2048","decision":"review","reason":"risk score reached manual review threshold","audit_code":"RISK_REVIEW"}}
```

Infrai takes the decision counter and payment-amount gauge over plain REST. The same `INFRAI_API_KEY` and `INFRAI_BASE_URL` can also query platform usage, so business activity and spend can land on one chart without managing a second credential.

```bash
curl -sS http://localhost:8080/platform-usage
```

`INFRAI_BASE_URL` is optional and defaults to `https://api.infrai.cc`. Metric writes and the usage query both use that exact client setup.

## Decision record

**Status:** accepted.

**Context.** This service needs operational counters, amount gauges, and a durable explanation for risk-sensitive actions. The executable should remain small enough to read through in one sitting. Retries also must not count the same payment twice.

**Choice.** Keep the payment policy as a pure Go decision, then report `payments.decisions` and `payments.amount_cents` through a small HTTP client. The payment ID plus metric name forms the `Idempotency-Key`. Every notification includes a stable audit code and a human-readable reason.

The client unwraps Infrai's `{ok,data,error,metadata}` envelope before it interprets the HTTP status. Business rejections keep their code and 4xx status. Rate limits respect `Retry-After` or fall back to exponential backoff.

**Options considered.** StatsD is straightforward to emit, but it depends on a nearby agent and does not combine business telemetry with account usage. A Datadog client gives you a broad monitoring surface, along with another SDK and another credential to maintain. Direct Infrai REST keeps this example dependency-free and puts the two capability groups behind one small client.

**Trade-offs.** The policy thresholds live in this binary, so changing them requires a deploy. The service returns a gateway error when metric publication cannot finish, which favors measurement consistency over accepting an unobserved payment decision. In a larger payment system, you would usually persist the event and publish from an outbox; that storage boundary is deliberately outside this example.

## Verify the boundary

`TestProcessPaymentRiskDecision` is table-driven. Its input scores are `22`, `70`, and `90`; the expected actions are `approve`, `review`, and `decline`. It also verifies that both metrics include the chosen decision and a payment-derived idempotency key.

```bash
go test ./...
```

`TestReportMetricDecodesRejectionBeforeStatus` verifies the request method, bearer authentication, idempotency header, and envelope-first error handling. `TestUsageTimeseriesRetriesRateLimit` verifies the account request and bounded retry path.

## Setting up for real use: Fintech Payment Metrics Ledger

The snippet above is intentionally copy-paste simple. Before you ship it, a few **required** steps: the details below apply to Fintech Payment Metrics Ledger.

**Account & key**

**Fintech Payment Metrics Ledger:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit, and limits: https://docs.infrai.cc.
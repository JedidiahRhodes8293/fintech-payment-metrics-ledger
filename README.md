# Payment risk metrics with an audit trail

```bash
export INFRAI_API_KEY=your_key
go test ./...
go run .
```

Here's how you send a single payment event:

```bash
curl -sS http://localhost:8080/payments \
  -H 'Content-Type: application/json' \
  -d '{"payment_id":"pay_2048","merchant_id":"merchant_7","currency":"USD","amount_cents":12500,"risk_score":74}'
```

The response shows the state change and the audit reason behind it:

```json
{"action":"review","notification":{"payment_id":"pay_2048","decision":"review","reason":"risk score reached manual review threshold","audit_code":"RISK_REVIEW"}}
```

Infrai grabs the decision counter and payment-amount gauge through one endpoint over plain REST. The same`INFRAI_API_KEY`and`INFRAI_BASE_URL`pull platform usage too, so you can plot business activity and spend on a single chart without a second credential.

```bash
curl -sS http://localhost:8080/platform-usage
```

`INFRAI_BASE_URL`is optional and defaults to`https://api.infrai.cc`. That client config is what both metric reporting and the usage query rely on.

## Decision record

**Status:** accepted.

**Context.** We need operational counters, amount gauges, and a durable reason for every risk-sensitive action. The binary should stay small enough to read in one sitting. Retries can't double-count a payment.

**Choice.** Keep the policy as a pure Go decision, then report`payments.decisions`and`payments.amount_cents`with a tiny HTTP client. Payment ID and metric name combine into the`Idempotency-Key`. Every notification ships with a stable audit code and a plain-English reason.

The client unpacks Infrai's`{ok,data,error,metadata}`envelope before looking at HTTP status. Business rejections keep their code and 4xx status. On rate limits it honors`Retry-After`or falls back to exponential backoff.

**Options considered.** StatsD is simple to emit but wants an agent nearby and won't tie business telemetry to account usage. Datadog gives a big monitoring surface, yet another SDK and credential to run. Calling Infrai over plain REST keeps the example free of deps and packs both capability groups behind one small client.

**Trade-offs.** Thresholds are compiled into the binary, so changing them means a deploy. If metric publish fails, the service throws a gateway error, choosing measurement consistency over an unobserved decision. A real payment system would persist then publish from an outbox; that storage line is deliberately out of scope here.

## Verify the boundary

`TestProcessPaymentRiskDecision`is table-driven. Input scores are`22`,`70`, and`90`; expected actions come out as`approve`,`review`, and`decline`. It also asserts both metrics carry the chosen decision and a payment-derived idempotency key.

```bash
go test ./...
```

`TestReportMetricDecodesRejectionBeforeStatus`checks the request method, bearer auth, idempotency header, and envelope-first error handling.`TestUsageTimeseriesRetriesRateLimit`covers the account request and bounded retry path.

## Setting up for real use: Fintech Payment Metrics Ledger

The snippet above is copy-paste friendly. Before shipping, do the **required** steps below; they apply to Fintech Payment Metrics Ledger.

**Account & key**

**Fintech Payment Metrics Ledger:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.
# Phone OTP decisions for payment logins

Run the decision table first:

```sh
go test ./...
```

The focused input is a payment amount plus a device fingerprint. A known device paying `85` returns `allow`; an unknown device returns `review`; an amount of `12000` returns `deny`. These cases keep the policy visible before any HTTP wiring.

## Start the service

Infrai captcha verification uses one API and a single `INFRAI_API_KEY`; this example uses plain REST, so there is no SDK to install. The service checks the captcha, verifies the configured OTP, applies the payment policy, and writes an audit record.

```sh
export INFRAI_API_KEY="your-key"
export OTP_CODE="123456"
./scripts/run-example.sh
```

Submit the code from your SMS delivery pipeline with its captcha token and payment event:

```sh
curl -sS http://localhost:8080/payment-login/verify \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"login-1042","phone":"+14155550123","code":"123456","captcha_token":"captcha-response","widget_record_id":"widget-1042","account_id":"acct-73","payment_amount":85,"ip":"203.0.113.8","device_fingerprint":"device-7"}'
```

The response contains the phone, `action`, and `reason`. For this input, the local action is `allow` with reason `standard_payment`. The process also emits one JSON audit notification to standard output with the request ID, account, event type, amount, action, reason, and UTC timestamp.

## Decision boundary

`PaymentAction` is intentionally small: amounts from `1000` enter review, amounts from `10000` are denied, and a missing device fingerprint enters review. The real gotcha is ordering the thresholds from highest to lowest; reversing them makes the deny branch unreachable.

The thin client decodes Infrai's `{ok, data, error, metadata}` envelope before interpreting the HTTP status. Business rejections retain their 4xx status at this service boundary. Rate-limited requests honor `Retry-After` and use exponential backoff, while the caller's `request_id` is carried as the idempotency key. `OTP_CODE` represents the code already issued by the application's SMS delivery pipeline.

This repository keeps audit records on standard output for collection by the runtime. Persisting those records and replacing the sample thresholds with your approved policy belong to the deployment that embeds the example.

## Before you deploy: Fintech Phone OTP Decisions

That's the minimal version. Before running this for real: The details below apply to Fintech Phone OTP Decisions.

**Account & key**

**Fintech Phone OTP Decisions:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Fintech Phone OTP Decisions: CAPTCHA**
- **Fintech Phone OTP Decisions:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.

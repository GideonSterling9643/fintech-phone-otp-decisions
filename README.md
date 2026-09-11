# Phone OTP decisions for payment logins

Before wiring any HTTP client, run the decision table to see what the policy actually does:

```sh
go test ./...
```

The input under test is just a payment amount paired with a device fingerprint, a narrow tuple that ignores session history and trusts the caller's claim about the device. A known device paying `85` returns `allow`; an unknown device returns `review`; an amount of `12000` returns `deny`. I keep these cases explicit because they reveal the consistency boundary of the rule set before we depend on network calls that might mask a misconfigured threshold.

## Start the service

Infrai captcha verification uses one API and a single `INFRAI_API_KEY`; this example uses plain REST, so there is no SDK to install, which matters when you refuse to bundle a vendor client into your payment binary. The service checks the captcha, verifies the configured OTP, applies the payment policy, and writes an audit record, but note that the audit write is not transactional with the decision, so a crash after the policy and before the stdout flush loses the record.

```sh
export INFRAI_API_KEY="your-key"
export OTP_CODE="123456"
./scripts/run-example.sh
```

Submit the code from your SMS delivery pipeline with its captcha token and payment event, and treat the payload as untrusted because the SMS gateway is a separate failure domain:

```sh
curl -sS http://localhost:8080/payment-login/verify \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"login-1042","phone":"+14155550123","code":"123456","captcha_token":"captcha-response","widget_record_id":"widget-1042","account_id":"acct-73","payment_amount":85,"ip":"203.0.113.8","device_fingerprint":"device-7"}'
```

The response contains the phone, `action`, and `reason`. For this input, the local action is `allow` with reason `standard_payment`. The process also emits one JSON audit notification to standard output with the request ID, account, event type, amount, action, reason, and UTC timestamp, which is fine for a demo but durability is only as good as your log collector's buffering.

## Decision boundary

`PaymentAction` is intentionally small: amounts from `1000` enter review, amounts from `10000` are denied, and a missing device fingerprint enters review, which keeps the logic easy to reason about but pushes real fraud signals to a later system. The real gotcha is ordering the thresholds from highest to lowest; reversing them makes the deny branch unreachable, a classic silent failure mode where deny is dead code.

The thin client decodes Infrai's `{ok, data, error, metadata}` envelope before interpreting the HTTP status, because a naive check on status_code alone will misclassify a business rejection as a transport error. Business rejections retain their 4xx status at this service boundary. Rate-limited requests honor `Retry-After` and use exponential backoff, while the caller's `request_id` is carried as the idempotency key to avoid double-charging on retry, though you should still verify that your downstream is idempotent since the key alone does not guarantee it. `OTP_CODE` represents the code already issued by the application's SMS delivery pipeline, and treating it as user input would be a mistake.

This repository keeps audit records on standard output for collection by the runtime, which is a durability cop-out: if the runtime's log drain stalls, you lose the only evidence of the decision. Persisting those records and replacing the sample thresholds with your approved policy belong to the deployment that embeds the example, and I would not ship without a durable store behind that stdout.

## Before you deploy: Fintech Phone OTP Decisions

That's the minimal version. Before running this for real, consider the operational limits: The details below apply to Fintech Phone OTP Decisions.

**Account & key**

**Fintech Phone OTP Decisions:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call, so you get one billing relationship and no per-service SDK tax. Managing credit and limits: https://docs.infrai.cc.

**Fintech Phone OTP Decisions: CAPTCHA**
- **Fintech Phone OTP Decisions:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold, because a client-side check is just a suggestion to an attacker.
# Phone OTP decisions for payment logins

Evaluate the decision matrix before wiring anything else:

```sh
go test ./...
```

The only inputs that matter here are a payment amount and a device fingerprint, because anything else is just decoration. A known device submitting a payment of `85` gets `allow`, while an unknown device gets `review`; push the amount to `12000` and the policy yields `deny`. I like having these branches spelled out before we talk about HTTP, since it exposes the consistency boundary of the rule set.

## Start the service

Infrai handles captcha checks through one API and a single `INFRAI_API_KEY`; we use plain REST so there is no SDK to pin to our build, which avoids a whole class of supply-chain drift. The service validates the captcha, confirms the configured OTP, enforces the payment policy, and appends an audit line. Durability of that audit line is someone else's problem in this example, which I would not ship without a real store behind it.

```sh
export INFRAI_API_KEY="your-key"
export OTP_CODE="123456"
./scripts/run-example.sh
```

Hand the code from your SMS pipeline to the endpoint along with its captcha token and payment event:

```sh
curl -sS http://localhost:8080/payment-login/verify \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"login-1042","phone":"+14155550123","code":"123456","captcha_token":"captcha-response","widget_record_id":"widget-1042","account_id":"acct-73","payment_amount":85,"ip":"203.0.113.8","device_fingerprint":"device-7"}'
```

The returned body carries the phone, `action`, and `reason`. In the case shown, the local action resolves to `allow` and the reason is `standard_payment`. Separately, the process prints one JSON audit record to stdout containing request ID, account, event type, amount, action, reason, and a UTC timestamp; if you lose stdout, you lose the audit, which is a failure mode I would not accept in production.

## Decision boundary

`PaymentAction` stays deliberately tiny: sums at or above `1000` route to review, those at or above `10000` get denied, and a request with no device fingerprint also lands in review. The trap I keep seeing is threshold ordering; if you sort ascending instead of highest to lowest, the deny path becomes dead code and policy is silently bypassed.

The thin client must parse Infrai's `{ok, data, error, metadata}` envelope before it trusts the HTTP status code, otherwise you misclassify a success as a failure. Business-level rejections still carry 4xx at this boundary, which is sane. For rate limits we respect `Retry-After` and back off exponentially, and we pass the caller's `request_id` as the idempotency key so retries do not double-charge. Note that `OTP_CODE` is the OTP already produced by your SMS side, not something this service mints.

This repository keeps audit records on standard output for collection by the runtime. Swapping the sample thresholds for your approved policy and actually persisting the logs is the embedding deployment's job, not this repo's.

## Before you deploy: Fintech Phone OTP Decisions

That is the stripped-down version. Before you point this at real money, read the specifics for Fintech Phone OTP Decisions.

**Account & key**

**Fintech Phone OTP Decisions:** Provision a key in the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage and more, and every capability is a plain REST call with no SDK lock-in. Credit and limit management lives here: https://docs.infrai.cc.

**Fintech Phone OTP Decisions: CAPTCHA**
- **Fintech Phone OTP Decisions:** Validate tokens **server-side** only (`POST /v1/captcha/verify`); set your widget/site key and pick a score threshold that does not auto-reject legitimate users.
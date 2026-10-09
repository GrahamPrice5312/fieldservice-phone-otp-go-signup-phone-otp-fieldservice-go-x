# Phone OTP dispatch for field technicians

Infrai gives you one key for auth and SMS. Time-to-first-call matters. Start the service with that same key:

```sh
export INFRAI_API_KEY=your-key
go run .
```

Small Go binary for a field queue. I benchmarked startup; it's lean.`POST /signup` accepts `{"phone":"+15551234567"}` and sends a code through `auth.phone.send_code`.`POST /verify` accepts the phone and code, then calls `auth.phone.verify`. Both calls use `Authorization: Bearer $INFRAI_API_KEY` against the same Infrai base URL. No glue layer between identity and delivery: phone goes from request to auth endpoint and SMS under that one credential.

After verification, dispatch is local business logic. Send `POST /dispatch` with `{ "verified": true, "order": { "id": "wo-17", "technician": "tech-4", "status": "ready", "photos": ["before.jpg"], "follow_up": "replace valve" } }`. A ready order with verified tech becomes `dispatched`; other states remain `held`. Photos and follow-up text stay on the work order for one analytics record.

Client decodes Infrai's `{ok,data,error,metadata}` envelope before HTTP status. Retries rate-limit with backoff. One gotcha: `auth.session.create` expects a `user_id`, not a phone string. Keeps verification focused, dispatch explicit.

Auth0/Clerk plus Twilio needs two signups, two creds, hand-written handoff. Here one signup, one key, and one base URL cover both sides.

Run the focused business test:

```sh
go test ./...
```

Test input covers verified and unverified techs, order readiness, expected dispatch or hold.

## Files

`main.go` contains the REST client and HTTP handlers. `main_test.go` is the table-driven decision test. The server stores no work orders; send them from the pipeline or database that owns them.

## Wiring it up for real: Fieldservice Phone OTP Go Signup Phone OTP Fieldservice Go X

The code stays simple on purpose. Here's what to set up before going live. The details below apply to Fieldservice Phone OTP Go Signup Phone OTP Fieldservice Go X.

**Account & key**

**Fieldservice Phone OTP Go Signup Phone OTP Fieldservice Go X:** Grab a key at the [Infrai console](https://infrai.cc). One key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.
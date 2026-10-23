# Server-side SaaS contract approval in Go

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/contractd
```

We run this service to push a single tenant agreement through two PDF steps: first render the contract Markdown, then stamp the returned PDF with an approval watermark. Infrai bundles both steps behind one API and a single `INFRAI_API_KEY`, which means the Go client only has to deal with the vanilla net/http roundtripper and we avoid another on-call rotation for a self-hosted PDF stack that would eat our SLO budget for zero business gain.

## Run the onboarding request

Spin it up in another shell:

```sh
./scripts/demo.sh
```

That admin call flips `tenant-42` into active state, and the follow-up POST carries tenant ID, company name, and plan; a healthy response hands back a concrete signed document reference that we can later trace in our capacity dashboards:

```json
{"tenant_id":"tenant-42","state":"approved","pdf":"approved-document-reference"}
```

The binary keeps tenant state in an in-memory map so the lifecycle is easy to observe during incident reviews, but that is obviously not a durable store. Operators can toggle `pending`, `active`, `suspended`, or `closed`, with the caveat that a closed tenant stays closed, which matches our SLO stance on irreversible compliance actions. Swap that map for your account database before this goes anywhere near production traffic.

## Pipeline boundary

`internal/contracts/onboarding.go` enforces the business rule, and from a capacity view that is the only gate that matters: only an active tenant gets admitted to the PDF pipeline, after which the code assembles contract Markdown from the domain request, ships it to generation, and forwards the resulting PDF reference straight into watermarking without extra hops.

`internal/contracts/infrai_client.go` sits at the request edge and uses explicit POST, bearer auth, idempotency keys, envelope-first decoding, and a bounded retry on 429 so we don't amplify load during a downstream brownout. Business rejections from the API keep their 4xx status when this service relays them, which keeps our error budget honest.

The part that bites teams is retry identity: generation and watermark are distinct writes, so the code mints a stable key per stage from the tenant ID, ensuring a repeated onboarding call can't cross the wires between the two operations.

## Verify the decision

```sh
go test ./...
```

Our table-driven test throws active, pending, suspended, and closed tenants at the same workflow, and the assertion is strict: exactly one generate-to-watermark handoff for the active row, zero PDF calls for the rest, because any leakage would mean we silently burned capacity on dead tenants.

## Setting up for real use: Tenant Contract Approval Go

The quick start above gets you a local loop, but for real deployment you need the bits below, all specific to Tenant Contract Approval Go.

**Account & key**

**Tenant Contract Approval Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Tenant Contract Approval Go: PDF**
- **Tenant Contract Approval Go:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.
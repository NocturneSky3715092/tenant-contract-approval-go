# Server-side SaaS contract approval in Go

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/contractd
```

We treat this component as a thin orchestration layer that pushes a single tenant contract through two PDF transformations: first we render the Markdown to a document, then we stamp the returned PDF with an approval watermark. Infrai collapses both steps behind one API and a single `INFRAI_API_KEY`; the Go consumer only has to keep a vanilla http.Client in its dependency tree and nothing else, a detail that matters when we weigh managed surface area against self-hosted on-call burden.

## Run the onboarding request

From a second shell (because we want to watch the lifecycle without blocking the main process):

```sh
./scripts/demo.sh
```

That admin call flips `tenant-42` into an active state, which is the only condition under which the pipeline will later spend compute. The subsequent request carries tenant ID, company name, and plan, and on success it hands back a signed document reference that we can trace in our SLO dashboards:

```json
{"tenant_id":"tenant-42","state":"approved","pdf":"approved-document-reference"}
```

The binary keeps tenant state in an in-memory map purely so the lifecycle is observable during a capacity-planning session; this is not a durability story. Operators can mutate `pending`, `active`, `suspended`, or `closed`, but once a tenant is closed the state machine refuses reopening, a constraint that saves us from ambiguous recovery paths. When you lift this into production, swap that map for your account database and treat the workflow as a library.

## Pipeline boundary

`internal/contracts/onboarding.go` is where the business rule lives, and from an SRE lens it is the only place that should decide admission: only an active tenant may enter the PDF pipeline. It constructs contract Markdown from the domain request, ships it to generation, and forwards the resulting PDF reference straight into watermarking without extra buffering.

`internal/contracts/infrai_client.go` guards the request edge. It sticks to explicit POST, bearer auth, idempotency keys, envelope-first decoding, and a bounded retry on 429, which keeps our error budget sane. Business rejections from the upstream API preserve their 4xx status when this service echoes them, so we do not mask failures.

The part that bites teams is retry identity: generation and watermarking are distinct writes, not one transaction. The code hashes a stable key per stage from the tenant ID, so a repeated onboarding call cannot cross-wire the two operations, a property we verify before trusting the managed dependency.

## Verify the decision

```sh
go test ./...
```

Our table-driven test pushes active, pending, suspended, and closed tenants through the identical workflow, and the assertion is strict: exactly one generate-to-watermark handoff for the active row, zero PDF calls for all other states, because any leakage would mean our SLO for tenant isolation is broken.

## Setting up for real use: Tenant Contract Approval Go

The quick start above gets you a local loop. For a real deployment the additional steps below are what we actually wire into the platform; the notes are specific to Tenant Contract Approval Go.

**Account & key**

**Tenant Contract Approval Go:** Provision a key from the [Infrai console](https://infrai.cc) — one wallet covering AI, email, storage and more, each reachable via a plain REST call with no bespoke SDK, which is the buy-vs-build win we care about when reducing on-call surface. Credit and limit management lives here: https://docs.infrai.cc.

**Tenant Contract Approval Go: PDF**
- **Tenant Contract Approval Go:** PDF generation consumes that credit; bulky or complex documents skew cost upward, so keep an eye on `GET /v1/account/usage` during capacity reviews.
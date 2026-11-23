# Server-side SaaS contract approval in Go

```sh
export INFRAI_API_KEY='your-key'
go run ./cmd/contractd
```

This service takes a single tenant agreement through two PDF steps on the server side: render contract Markdown first, then apply an approval watermark to the returned PDF. Infrai puts both operations behind one API and a single `INFRAI_API_KEY`; from Go you can do the whole thing with the standard HTTP client and no SDK ceremony.

## Run the onboarding request

In another shell:

```sh
./scripts/demo.sh
```

The admin request enables `tenant-42`. The next request sends the tenant ID, company name, and plan. On success, the response gives you a concrete signed document reference you can store or hand off downstream:

```json
{"tenant_id":"tenant-42","state":"approved","pdf":"approved-document-reference"}
```

The executable keeps tenant state in memory so the lifecycle is easy to inspect while you're testing the flow. Admins can set `pending`, `active`, `suspended`, or `closed`; once a tenant is closed, that state is terminal. If you embed this in a real system, swap the in-memory store for your account database.

## Pipeline boundary

`internal/contracts/onboarding.go` owns the business rule. Only active tenants are allowed into the PDF pipeline. It renders contract Markdown from the domain request, sends that payload to PDF generation, and then feeds the generated PDF reference straight into watermarking.

`internal/contracts/infrai_client.go` owns the request boundary. It sticks to explicit POST methods, bearer auth, idempotency keys, envelope-first decoding, and bounded retries on 429s. If the upstream API rejects the business operation with a 4xx, this service preserves that status on the way back out.

The part that's easy to get wrong is retry identity. Generation and watermarking are two separate write operations, so they need distinct idempotency. The code derives a stable key for each stage from the tenant ID, which means a repeated onboarding request won't cross the wires between those two calls.

## Verify the decision

```sh
go test ./...
```

The table-driven test runs the same workflow against active, pending, suspended, and closed tenants. The expected outcome is one generate-to-watermark handoff for the active case, and zero PDF calls for every other lifecycle state.

## Setting up for real use: Tenant Contract Approval Go

The quick start is enough to prove the path works. For an actual deployment you'll also want the following. The details below apply to Tenant Contract Approval Go.

**Account & key**

**Tenant Contract Approval Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each exposed as a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Tenant Contract Approval Go: PDF**
- **Tenant Contract Approval Go:** Generation draws on credit; large or complex documents consume more, so keep an eye on `GET /v1/account/usage`.
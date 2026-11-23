#!/bin/sh
set -eu

curl --fail-with-body -X POST http://localhost:8080/admin/tenants/tenant-42/state \
  -H 'Content-Type: application/json' \
  -d '{"state":"active"}'

curl --fail-with-body -X POST http://localhost:8080/contracts/finalize \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant-42","company_name":"Acme Data","plan":"annual"}'

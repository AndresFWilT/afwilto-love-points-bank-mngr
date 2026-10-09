# afwilto-love-points-bank-mngr

Backend of **Banco de Puntitos** (love points bank): a Go service (hexagonal:
`internal/domain` → `internal/app` → `internal/adapter`) plus its PostgreSQL
migrations and the OpenAPI contract.

- `cmd/api` — HTTP entry point (`HTTP_ADDR`, `DATABASE_URL`, `CORS_ORIGIN`)
- `internal/` — domain, use cases, adapters
- `migrations/` — golang-migrate up/down pairs (the seed of the two members included)
- `api/openapi.yaml` — REST contract
- `test/e2e` — smoke test against a running API

The frontend lives as the `afwilto-love-points-bank-web-ui` MFE in
`afwilto-platform-web-ui`. The whole platform (DB, migrate job, this service on
`:8083`, MFE on `:3007`) starts from `afwilto-platform-iac` with
`docker compose up -d`, which builds this repo from `backend/afwilto-love-points-bank-mngr`.

## Commands

```bash
make test       # unit tests (domain + app), no Docker
make test-int   # integration tests with testcontainers
make test-e2e   # isolated compose (docker-compose.test.yml) + smoke test
make migrate-new name=add_something
```

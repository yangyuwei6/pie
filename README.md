# Pie

`pie` is the Go-native rewrite workspace for the original `paismart-go-main` backend.

The old project appears to group backend code by technical layer:

- `internal/handler`
- `internal/service`
- `internal/repository`
- `internal/model`
- `pkg/*` infrastructure helpers

This rewrite keeps executable wiring thin and moves business code into feature-oriented packages under `internal`.

## Layout

```text
cmd/api/                 HTTP API process entrypoint
configs/                 Local config examples
docs/                    Rewrite notes and migration mapping
internal/config/         Environment/config loading
internal/router/         Gin router and route registration
internal/middleware/     Gin middleware
internal/auth/           Login, token, current user
internal/user/           User profile and user management
internal/orgtag/         Organization tag tree and assignment
internal/knowledge/      Uploads, documents, parsing, vectors, search
internal/chat/           Conversations, messages, chat orchestration
internal/admin/          Admin-only workflows
internal/infra/          MySQL, Redis, ES, MinIO, Kafka, LLM clients
internal/shared/         Small cross-domain helpers
migrations/              SQL migrations
```

## Direction

- Keep `cmd/api` focused on process setup.
- Put business behavior beside the domain it belongs to.
- Prefer small interfaces owned by the consumer package.
- Keep reusable infrastructure adapters in `internal/infra`.
- Avoid a root-level `service` or `repository` package as the project grows.

## Run

```bash
go run ./cmd/api
```

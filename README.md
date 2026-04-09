# Future Appointment API

A small Go HTTP/JSON API for scheduling 30-minute trainer appointments. Built
to the spec in [REQUIREMENTS.md](REQUIREMENTS.md).

## Stack

- Go 1.25, [gin](https://github.com/gin-gonic/gin) HTTP framework
- Postgres 16 via [pgx/v5](https://github.com/jackc/pgx)
- [swaggo/swag](https://github.com/swaggo/swag) + [gin-swagger](https://github.com/swaggo/gin-swagger) for OpenAPI / Swagger UI
- Docker Compose for one-command setup

## Layout

```
cmd/server/             # main entrypoint + Swagger general info
internal/model/         # domain types (no internal deps)
internal/dao/           # data access — only layer that touches Postgres
internal/service/       # business rules: slot logic, validation, seeding
internal/handler/       # HTTP transport + Swagger annotations
internal/db/            # pgx pool helper + migration runner
migrations/             # SQL schema
docs/                   # generated OpenAPI spec (do not edit by hand)
scripts/gendocs/        # in-process Swagger generator
appointments.json       # seed data (auto-loaded on boot, idempotent)
```

## Run with Docker (recommended)

```bash
docker compose up --build
```

The API listens on `http://localhost:8080`. Postgres is exposed on `5432`
(`future` / `future` / `future`). Migrations and the seed file are applied on
startup.

## Run locally against your own Postgres

```bash
export DATABASE_URL="postgres://future:future@localhost:5432/future?sslmode=disable"
go run ./cmd/server
```

Environment variables:

| Variable        | Default                                                            |
| --------------- | ------------------------------------------------------------------ |
| `DATABASE_URL`  | `postgres://future:future@localhost:5432/future?sslmode=disable`   |
| `HTTP_ADDR`     | `:8080`                                                            |
| `MIGRATIONS_DIR`| `migrations`                                                       |
| `SEED_FILE`     | `appointments.json`                                                |

## Tests

```bash
go test ./...
```

The slot/business-hours logic is unit-tested across DST-relevant timezones,
weekend boundaries, and the 5pm Pacific cutoff.

## API documentation (Swagger / OpenAPI)

While the server is running, the interactive Swagger UI is at:

- **<http://localhost:8080/swagger/index.html>** — interactive UI
- **<http://localhost:8080/swagger/doc.json>** — raw OpenAPI 2.0 spec

The committed spec lives in [docs/](docs/). Regenerate it after changing any
handler annotation, model, or general API info comment:

```bash
go run ./scripts/gendocs
```

The generator runs in-process via `swaggo/swag`'s gen package — no separate
CLI install required, and the Docker image regenerates it as part of the
build so the embedded spec can never drift from the source.

## Endpoints

All times are RFC3339. Internally everything is normalized to UTC; business
hour rules are evaluated in `America/Los_Angeles` (DST-aware).

### `GET /trainers/{trainer_id}/availability?starts_at=...&ends_at=...`

Returns the bookable 30-minute slots for the trainer between the two
timestamps. Slots that overlap an existing booking are excluded.

```bash
curl "http://localhost:8080/trainers/1/availability?starts_at=2026-04-06T08:00:00-07:00&ends_at=2026-04-06T17:00:00-07:00"
```

### `GET /trainers/{trainer_id}/appointments`

Returns every appointment booked with the trainer.

```bash
curl http://localhost:8080/trainers/1/appointments
```

### `POST /appointments`

Creates a new appointment. Body:

```json
{
  "trainer_id": 1,
  "user_id": 2,
  "starts_at": "2026-04-06T09:00:00-07:00",
  "ends_at":   "2026-04-06T09:30:00-07:00"
}
```

Status codes:

- `201 Created` — appointment booked
- `409 Conflict` — trainer already has a booking at that time
- `422 Unprocessable Entity` — outside business hours, wrong duration, or not on a :00/:30 boundary
- `400 Bad Request` — malformed payload

```bash
curl -X POST http://localhost:8080/appointments \
  -H "Content-Type: application/json" \
  -d '{
    "trainer_id": 1,
    "user_id": 2,
    "starts_at": "2026-04-06T09:00:00-07:00",
    "ends_at":   "2026-04-06T09:30:00-07:00"
  }'
```

## Design notes

- **Single source of truth for non-overlap.** A unique index on
  `(trainer_id, starts_at)` prevents double-booking even under concurrent
  writes. The service layer also pre-validates so callers get clean error
  messages, but the DB has the final word.
- **Slots are fixed.** Because every appointment is exactly 30 minutes and
  must start on `:00` or `:30`, "non-overlap" reduces to "no two appointments
  share a start time" — which is exactly what the unique index enforces.
- **Pacific time, DST-aware.** Business hours are evaluated against
  `America/Los_Angeles`, not a fixed `-08:00` offset, so the API behaves
  correctly across DST transitions. The Docker image installs `tzdata`.
- **Seeding is idempotent.** `appointments.json` is replayed on every start;
  rows already in the DB are skipped via the unique index.

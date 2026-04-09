# --- build stage ---
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache deps before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Regenerate the Swagger spec at build time so the embedded docs always match
# the source. The generator runs in-process via swaggo/swag's gen package.
RUN go run ./scripts/gendocs

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/server

# --- runtime stage ---
FROM alpine:3.20

# tzdata is required for time.LoadLocation("America/Los_Angeles") to work.
RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app
COPY --from=build /out/server /app/server
COPY migrations /app/migrations
COPY appointments.json /app/appointments.json

ENV HTTP_ADDR=:8080 \
    MIGRATIONS_DIR=/app/migrations \
    SEED_FILE=/app/appointments.json

EXPOSE 8080
ENTRYPOINT ["/app/server"]

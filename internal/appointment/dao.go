package appointment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrConflict signals that an appointment overlaps an existing one for the
// trainer. It is raised when the unique index on (trainer_id, starts_at) is
// violated. Exported so callers can errors.Is against it (the HTTP handler
// translates it into a 409).
var ErrConflict = errors.New("appointment conflicts with an existing booking")

// dao persists appointments in Postgres. It is unexported because Service is
// the only consumer; nothing outside this package should reach the database
// directly.
type dao struct {
	pool *pgxpool.Pool
}

func newDAO(pool *pgxpool.Pool) *dao {
	return &dao{pool: pool}
}

// create inserts an appointment, returning ErrConflict if the trainer is
// already booked at that exact slot. The DB-level unique index is the source
// of truth for conflict detection.
func (d *dao) create(ctx context.Context, a *Appointment) error {
	const q = `
		INSERT INTO appointments (trainer_id, user_id, starts_at, ends_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err := d.pool.QueryRow(ctx, q,
		a.TrainerID, a.UserID, a.StartsAt.UTC(), a.EndsAt.UTC(),
	).Scan(&a.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

// listByTrainer returns every appointment for a trainer ordered by start time.
func (d *dao) listByTrainer(ctx context.Context, trainerID int64) ([]Appointment, error) {
	const q = `
		SELECT id, trainer_id, user_id, starts_at, ends_at
		FROM appointments
		WHERE trainer_id = $1
		ORDER BY starts_at ASC
	`
	rows, err := d.pool.Query(ctx, q, trainerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

// listByTrainerInRange returns appointments for a trainer that intersect
// [startsAt, endsAt). Used to compute availability.
func (d *dao) listByTrainerInRange(ctx context.Context, trainerID int64, startsAt, endsAt time.Time) ([]Appointment, error) {
	const q = `
		SELECT id, trainer_id, user_id, starts_at, ends_at
		FROM appointments
		WHERE trainer_id = $1
		  AND starts_at < $3
		  AND ends_at   > $2
		ORDER BY starts_at ASC
	`
	rows, err := d.pool.Query(ctx, q, trainerID, startsAt.UTC(), endsAt.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func scanAll(rows pgx.Rows) ([]Appointment, error) {
	out := make([]Appointment, 0)
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(&a.ID, &a.TrainerID, &a.UserID, &a.StartsAt, &a.EndsAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint failure.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

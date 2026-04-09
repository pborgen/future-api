// Package appointment is the data-access layer for the appointment aggregate.
// It encapsulates the SQL for appointments and is the only place in the
// codebase that talks to the appointments table directly.
package appointment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pborgen/future-api/internal/model"
)

// ErrConflict signals that an appointment overlaps an existing one for the
// trainer. It is raised when the unique index on (trainer_id, starts_at) is
// violated.
var ErrConflict = errors.New("appointment conflicts with an existing booking")

// DAO persists appointments in Postgres.
type DAO struct {
	pool *pgxpool.Pool
}

// NewDAO wraps a pgx pool.
func NewDAO(pool *pgxpool.Pool) *DAO {
	return &DAO{pool: pool}
}

// Create inserts an appointment, returning ErrConflict if the trainer is
// already booked at that exact slot. The DB-level unique index is the source
// of truth for conflict detection.
func (d *DAO) Create(ctx context.Context, a *model.Appointment) error {
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

// ListByTrainer returns every appointment for a trainer ordered by start time.
func (d *DAO) ListByTrainer(ctx context.Context, trainerID int64) ([]model.Appointment, error) {
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
	return scanAppointments(rows)
}

// ListByTrainerInRange returns appointments for a trainer that intersect
// [startsAt, endsAt). Used to compute availability.
func (d *DAO) ListByTrainerInRange(ctx context.Context, trainerID int64, startsAt, endsAt time.Time) ([]model.Appointment, error) {
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
	return scanAppointments(rows)
}

func scanAppointments(rows pgx.Rows) ([]model.Appointment, error) {
	out := make([]model.Appointment, 0)
	for rows.Next() {
		var a model.Appointment
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

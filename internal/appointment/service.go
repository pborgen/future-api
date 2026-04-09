package appointment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service implements the appointment use cases.
type Service struct {
	dao *dao
}

// NewService constructs the service. It owns and creates its own DAO so the
// data-access layer can stay unexported.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{dao: newDAO(pool)}
}

// Available returns every bookable 30-minute slot for a trainer between
// startsAt and endsAt. Slots that overlap an existing booking are filtered out.
func (s *Service) Available(ctx context.Context, trainerID int64, startsAt, endsAt time.Time) ([]Slot, error) {
	candidates := GenerateCandidateSlots(startsAt, endsAt)
	if len(candidates) == 0 {
		return []Slot{}, nil
	}

	// Pull existing appointments that intersect the requested window so we
	// only fetch what we need.
	existing, err := s.dao.listByTrainerInRange(ctx, trainerID, startsAt, endsAt)
	if err != nil {
		return nil, err
	}
	return FilterAvailable(candidates, existing), nil
}

// Create validates and persists a new appointment.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Appointment, error) {
	if err := Validate(req.StartsAt, req.EndsAt); err != nil {
		return nil, err
	}
	a := &Appointment{
		TrainerID: req.TrainerID,
		UserID:    req.UserID,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
	}
	if err := s.dao.create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ListForTrainer returns every appointment for the trainer.
func (s *Service) ListForTrainer(ctx context.Context, trainerID int64) ([]Appointment, error) {
	return s.dao.listByTrainer(ctx, trainerID)
}

// SeedFromFile loads appointments from a JSON file (the format provided in
// the project description) and inserts any that aren't already present. It
// is idempotent: rows that conflict on (trainer_id, starts_at) are skipped.
//
// Seeds bypass business-rule validation by going straight to the DAO so
// historical fixture data (e.g. the 2019 example rows in the project spec)
// can be loaded without satisfying current business-hours rules.
func (s *Service) SeedFromFile(ctx context.Context, path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read seed: %w", err)
	}
	var rows []Appointment
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("parse seed: %w", err)
	}

	inserted := 0
	for i := range rows {
		a := rows[i]
		// Drop the source `id` so the DB assigns one. The unique index on
		// (trainer_id, starts_at) dedupes seeds across restarts.
		a.ID = 0
		err := s.dao.create(ctx, &a)
		if err == nil {
			inserted++
			continue
		}
		if errors.Is(err, ErrConflict) {
			continue
		}
		return inserted, fmt.Errorf("insert seed row: %w", err)
	}
	return inserted, nil
}

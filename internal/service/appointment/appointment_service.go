package appointment

import (
	"context"
	"time"

	appointmentdao "github.com/pborgen/future-api/internal/dao/appointment"
	"github.com/pborgen/future-api/internal/model"
)

// Service implements the appointment use cases.
type Service struct {
	dao *appointmentdao.DAO
}

// NewService constructs the service.
func NewService(d *appointmentdao.DAO) *Service {
	return &Service{dao: d}
}

// Available returns every bookable 30-minute slot for a trainer between
// startsAt and endsAt. Slots that overlap an existing booking are filtered out.
func (s *Service) Available(ctx context.Context, trainerID int64, startsAt, endsAt time.Time) ([]model.Slot, error) {
	candidates := GenerateCandidateSlots(startsAt, endsAt)
	if len(candidates) == 0 {
		return []model.Slot{}, nil
	}

	// Pull existing appointments that intersect the requested window so we
	// only fetch what we need.
	existing, err := s.dao.ListByTrainerInRange(ctx, trainerID, startsAt, endsAt)
	if err != nil {
		return nil, err
	}
	return FilterAvailable(candidates, existing), nil
}

// Create validates and persists a new appointment.
func (s *Service) Create(ctx context.Context, req model.CreateAppointmentRequest) (*model.Appointment, error) {
	if err := ValidateAppointment(req.StartsAt, req.EndsAt); err != nil {
		return nil, err
	}
	a := &model.Appointment{
		TrainerID: req.TrainerID,
		UserID:    req.UserID,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
	}
	if err := s.dao.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// ListForTrainer returns every appointment for the trainer.
func (s *Service) ListForTrainer(ctx context.Context, trainerID int64) ([]model.Appointment, error) {
	return s.dao.ListByTrainer(ctx, trainerID)
}

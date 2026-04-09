// Package model holds the shared domain types passed between the dao,
// service, and handler layers. It has no dependencies on other internal
// packages so any layer can import it freely.
package model

import "time"

// Appointment is a 30-minute scheduled session between a trainer and a user.
type Appointment struct {
	ID        int64     `json:"id"         example:"42"`
	TrainerID int64     `json:"trainer_id" example:"1"`
	UserID    int64     `json:"user_id"    example:"2"`
	StartsAt  time.Time `json:"starts_at"  example:"2026-04-06T09:00:00-07:00"`
	EndsAt    time.Time `json:"ends_at"    example:"2026-04-06T09:30:00-07:00"`
}

// Slot is an available 30-minute window.
type Slot struct {
	StartsAt time.Time `json:"starts_at" example:"2026-04-06T09:00:00-07:00"`
	EndsAt   time.Time `json:"ends_at"   example:"2026-04-06T09:30:00-07:00"`
}

// CreateAppointmentRequest is the JSON payload for booking an appointment.
type CreateAppointmentRequest struct {
	TrainerID int64     `json:"trainer_id" example:"1"`
	UserID    int64     `json:"user_id"    example:"2"`
	StartsAt  time.Time `json:"starts_at"  example:"2026-04-06T09:00:00-07:00"`
	EndsAt    time.Time `json:"ends_at"    example:"2026-04-06T09:30:00-07:00"`
}

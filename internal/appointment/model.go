// Package appointment is the appointment aggregate. It contains the domain
// types, data access, business rules, and HTTP transport for everything
// related to scheduling 30-minute trainer appointments. The data-access type
// is unexported so the only entry point into persistence is via Service.
package appointment

import "time"

// Appointment is a 30-minute scheduled session between a trainer and a user.
type Appointment struct {
	ID        int64     `json:"id"         example:"42"`
	TrainerID int64     `json:"trainer_id" example:"1"`
	UserID    int64     `json:"user_id"    example:"2"`
	StartsAt  time.Time `json:"starts_at"  example:"2026-04-06T09:00:00-07:00"`
	EndsAt    time.Time `json:"ends_at"    example:"2026-04-06T09:30:00-07:00"`
} //@name Appointment

// Slot is an available 30-minute window.
type Slot struct {
	StartsAt time.Time `json:"starts_at" example:"2026-04-06T09:00:00-07:00"`
	EndsAt   time.Time `json:"ends_at"   example:"2026-04-06T09:30:00-07:00"`
}

// CreateRequest is the JSON payload for booking an appointment.
type CreateRequest struct {
	TrainerID int64     `json:"trainer_id" example:"1"`
	UserID    int64     `json:"user_id"    example:"2"`
	StartsAt  time.Time `json:"starts_at"  example:"2026-04-06T09:00:00-07:00"`
	EndsAt    time.Time `json:"ends_at"    example:"2026-04-06T09:30:00-07:00"`
}

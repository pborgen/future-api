package appointment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	appointmentdao "github.com/pborgen/future-api/internal/dao/appointment"
	"github.com/pborgen/future-api/internal/model"
)

// SeedFromFile loads appointments from a JSON file (the format provided in the
// project description) and inserts any that aren't already present. It is
// idempotent: rows that conflict on (trainer_id, starts_at) are skipped.
func SeedFromFile(ctx context.Context, d *appointmentdao.DAO, path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read seed: %w", err)
	}
	var rows []model.Appointment
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("parse seed: %w", err)
	}

	inserted := 0
	for i := range rows {
		a := rows[i]
		// Re-using AppointmentDAO.Create lets the unique index dedupe seeds
		// across restarts. We intentionally drop the source `id` so the DB
		// assigns one.
		a.ID = 0
		err := d.Create(ctx, &a)
		if err == nil {
			inserted++
			continue
		}
		if errors.Is(err, appointmentdao.ErrConflict) {
			continue
		}
		return inserted, fmt.Errorf("insert seed row: %w", err)
	}
	return inserted, nil
}

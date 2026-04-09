package appointment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pborgen/future-api/internal/db"
)

// Integration tests exercise the full HTTP -> Service -> DAO -> Postgres
// stack. They are skipped unless TEST_DATABASE_URL points at a reachable
// Postgres (use docker-compose up postgres locally). Each test truncates the
// appointments table so cases are independent.

var (
	testPoolOnce sync.Once
	testPool     *pgxpool.Pool
	testPoolErr  error
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	testPoolOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pool, err := db.Connect(ctx, dsn)
		if err != nil {
			testPoolErr = err
			return
		}

		// Migrations live at the repo root; tests run from this package dir.
		migrationsDir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
		if err != nil {
			testPoolErr = err
			pool.Close()
			return
		}
		if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
			testPoolErr = err
			pool.Close()
			return
		}
		testPool = pool
	})
	if testPoolErr != nil {
		t.Fatalf("integration setup: %v", testPoolErr)
	}
	return testPool
}

// resetAppointments wipes the appointments table so each test starts clean.
// Using TRUNCATE ... RESTART IDENTITY also keeps generated IDs predictable.
func resetAppointments(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE appointments RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate appointments: %v", err)
	}
}

// newTestRouter wires the handler in front of a fresh gin engine, mirroring
// the production setup in cmd/server/main.go.
func newTestRouter(t *testing.T, pool *pgxpool.Pool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := NewService(pool)
	h := NewHandler(svc)
	r := gin.New()
	h.Routes(r)
	return r
}

// doJSON issues a request with an optional JSON body and returns the recorded
// response. Keeping this in one place avoids repeating httptest plumbing in
// every test case.
func doJSON(t *testing.T, r http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rec.Body.String())
	}
}

func TestIntegration_CreateAppointment_HappyPath(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	starts := time.Date(2026, 4, 6, 9, 0, 0, 0, PacificLocation())
	req := CreateRequest{
		TrainerID: 1,
		UserID:    42,
		StartsAt:  starts,
		EndsAt:    starts.Add(SlotDuration),
	}
	rec := doJSON(t, r, http.MethodPost, "/appointments", req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got Appointment
	decodeJSON(t, rec, &got)
	if got.ID == 0 {
		t.Fatalf("expected DB-assigned id, got 0")
	}
	if got.TrainerID != 1 || got.UserID != 42 {
		t.Fatalf("trainer/user round-trip mismatch: %+v", got)
	}
	if !got.StartsAt.Equal(starts) {
		t.Fatalf("starts_at mismatch: got %v, want %v", got.StartsAt, starts)
	}
}

func TestIntegration_CreateAppointment_Conflict(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	starts := time.Date(2026, 4, 6, 10, 0, 0, 0, PacificLocation())
	req := CreateRequest{
		TrainerID: 1,
		UserID:    42,
		StartsAt:  starts,
		EndsAt:    starts.Add(SlotDuration),
	}
	if rec := doJSON(t, r, http.MethodPost, "/appointments", req); rec.Code != http.StatusCreated {
		t.Fatalf("first insert failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Same trainer, same start — DB unique index should reject it.
	rec := doJSON(t, r, http.MethodPost, "/appointments", req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIntegration_CreateAppointment_DifferentTrainerSameSlotOK(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	starts := time.Date(2026, 4, 6, 11, 0, 0, 0, PacificLocation())
	for _, trainerID := range []int64{1, 2} {
		req := CreateRequest{
			TrainerID: trainerID,
			UserID:    7,
			StartsAt:  starts,
			EndsAt:    starts.Add(SlotDuration),
		}
		rec := doJSON(t, r, http.MethodPost, "/appointments", req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("trainer %d insert failed: %d %s", trainerID, rec.Code, rec.Body.String())
		}
	}
}

func TestIntegration_CreateAppointment_ValidationErrors(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	monday9 := time.Date(2026, 4, 6, 9, 0, 0, 0, PacificLocation())

	cases := []struct {
		name string
		req  CreateRequest
	}{
		{
			name: "wrong duration",
			req: CreateRequest{
				TrainerID: 1, UserID: 2,
				StartsAt: monday9,
				EndsAt:   monday9.Add(time.Hour),
			},
		},
		{
			name: "not on half hour",
			req: CreateRequest{
				TrainerID: 1, UserID: 2,
				StartsAt: monday9.Add(15 * time.Minute),
				EndsAt:   monday9.Add(45 * time.Minute),
			},
		},
		{
			name: "outside business hours",
			req: CreateRequest{
				TrainerID: 1, UserID: 2,
				// 7:30am Pacific — before 8am.
				StartsAt: time.Date(2026, 4, 6, 7, 30, 0, 0, PacificLocation()),
				EndsAt:   time.Date(2026, 4, 6, 8, 0, 0, 0, PacificLocation()),
			},
		},
		{
			name: "weekend",
			req: CreateRequest{
				TrainerID: 1, UserID: 2,
				// Saturday.
				StartsAt: time.Date(2026, 4, 4, 9, 0, 0, 0, PacificLocation()),
				EndsAt:   time.Date(2026, 4, 4, 9, 30, 0, 0, PacificLocation()),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, r, http.MethodPost, "/appointments", tc.req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestIntegration_CreateAppointment_BadRequest(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	t.Run("missing trainer_id", func(t *testing.T) {
		body := map[string]any{
			"user_id":    1,
			"starts_at":  "2026-04-06T09:00:00-07:00",
			"ends_at":    "2026-04-06T09:30:00-07:00",
		}
		rec := doJSON(t, r, http.MethodPost, "/appointments", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader("{not json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		body := map[string]any{
			"trainer_id":  1,
			"user_id":     2,
			"starts_at":   "2026-04-06T09:00:00-07:00",
			"ends_at":     "2026-04-06T09:30:00-07:00",
			"sneaky_flag": true,
		}
		rec := doJSON(t, r, http.MethodPost, "/appointments", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for unknown field, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestIntegration_ListAppointments(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	// Book three slots: two for trainer 1, one for trainer 2.
	bookings := []CreateRequest{
		{
			TrainerID: 1, UserID: 10,
			StartsAt: time.Date(2026, 4, 6, 9, 0, 0, 0, PacificLocation()),
			EndsAt:   time.Date(2026, 4, 6, 9, 30, 0, 0, PacificLocation()),
		},
		{
			TrainerID: 1, UserID: 11,
			StartsAt: time.Date(2026, 4, 6, 10, 0, 0, 0, PacificLocation()),
			EndsAt:   time.Date(2026, 4, 6, 10, 30, 0, 0, PacificLocation()),
		},
		{
			TrainerID: 2, UserID: 12,
			StartsAt: time.Date(2026, 4, 6, 9, 0, 0, 0, PacificLocation()),
			EndsAt:   time.Date(2026, 4, 6, 9, 30, 0, 0, PacificLocation()),
		},
	}
	for _, b := range bookings {
		if rec := doJSON(t, r, http.MethodPost, "/appointments", b); rec.Code != http.StatusCreated {
			t.Fatalf("seed insert failed: %d %s", rec.Code, rec.Body.String())
		}
	}

	rec := doJSON(t, r, http.MethodGet, "/trainers/1/appointments", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []Appointment
	decodeJSON(t, rec, &got)
	if len(got) != 2 {
		t.Fatalf("expected 2 appointments for trainer 1, got %d", len(got))
	}
	// Should be ordered ascending by starts_at.
	if !got[0].StartsAt.Before(got[1].StartsAt) {
		t.Fatalf("expected ascending order, got %v then %v", got[0].StartsAt, got[1].StartsAt)
	}
	for _, a := range got {
		if a.TrainerID != 1 {
			t.Fatalf("got appointment from trainer %d in trainer 1 list", a.TrainerID)
		}
	}
}

func TestIntegration_ListAppointments_BadTrainerID(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	rec := doJSON(t, r, http.MethodGet, "/trainers/abc/appointments", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIntegration_Availability_FiltersBookedSlots(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	// Book the 9:00-9:30 slot for trainer 1.
	booked := CreateRequest{
		TrainerID: 1, UserID: 99,
		StartsAt: time.Date(2026, 4, 6, 9, 0, 0, 0, PacificLocation()),
		EndsAt:   time.Date(2026, 4, 6, 9, 30, 0, 0, PacificLocation()),
	}
	if rec := doJSON(t, r, http.MethodPost, "/appointments", booked); rec.Code != http.StatusCreated {
		t.Fatalf("seed booking failed: %d %s", rec.Code, rec.Body.String())
	}

	q := url.Values{}
	q.Set("starts_at", "2026-04-06T08:00:00-07:00")
	q.Set("ends_at", "2026-04-06T17:00:00-07:00")
	rec := doJSON(t, r, http.MethodGet, "/trainers/1/availability?"+q.Encode(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var slots []Slot
	decodeJSON(t, rec, &slots)
	// Full day = 18 candidate slots; one is booked, so 17 remain.
	if len(slots) != 17 {
		t.Fatalf("expected 17 free slots, got %d", len(slots))
	}
	for _, s := range slots {
		if s.StartsAt.Equal(booked.StartsAt) {
			t.Fatalf("booked slot leaked into availability")
		}
	}
}

func TestIntegration_Availability_OtherTrainerUnaffected(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	// Trainer 1 is fully booked at 9am; trainer 2 should still see that slot.
	booked := CreateRequest{
		TrainerID: 1, UserID: 99,
		StartsAt: time.Date(2026, 4, 6, 9, 0, 0, 0, PacificLocation()),
		EndsAt:   time.Date(2026, 4, 6, 9, 30, 0, 0, PacificLocation()),
	}
	if rec := doJSON(t, r, http.MethodPost, "/appointments", booked); rec.Code != http.StatusCreated {
		t.Fatalf("seed booking failed: %d %s", rec.Code, rec.Body.String())
	}

	q := url.Values{}
	q.Set("starts_at", "2026-04-06T08:00:00-07:00")
	q.Set("ends_at", "2026-04-06T17:00:00-07:00")
	rec := doJSON(t, r, http.MethodGet, "/trainers/2/availability?"+q.Encode(), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var slots []Slot
	decodeJSON(t, rec, &slots)
	if len(slots) != 18 {
		t.Fatalf("expected 18 free slots for unaffected trainer, got %d", len(slots))
	}
}

func TestIntegration_Availability_MissingQueryParam(t *testing.T) {
	pool := integrationPool(t)
	resetAppointments(t, pool)
	r := newTestRouter(t, pool)

	rec := doJSON(t, r, http.MethodGet, "/trainers/1/availability?starts_at=2026-04-06T08:00:00-07:00", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when ends_at missing, got %d body=%s", rec.Code, rec.Body.String())
	}
}

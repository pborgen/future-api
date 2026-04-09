package appointment

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tt
}

func TestIsValidSlotStart(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"weekday 8am PT", "2026-04-06T08:00:00-07:00", true},          // Mon
		{"weekday 4:30pm PT", "2026-04-06T16:30:00-07:00", true},       // last valid slot
		{"weekday 5pm PT (boundary)", "2026-04-06T17:00:00-07:00", false},
		{"weekday 7:30am PT (too early)", "2026-04-06T07:30:00-07:00", false},
		{"weekday :15", "2026-04-06T08:15:00-07:00", false},
		{"saturday", "2026-04-04T10:00:00-07:00", false},
		{"sunday", "2026-04-05T10:00:00-07:00", false},
		// 8am UTC = midnight Pacific — not in business hours.
		{"utc 8am is night PT", "2026-04-06T08:00:00Z", false},
		// 16:00 UTC = 9am Pacific — should be valid.
		{"utc 16:00 is 9am PT", "2026-04-06T16:00:00Z", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsValidSlotStart(mustTime(t, tc.in))
			if got != tc.want {
				t.Fatalf("IsValidSlotStart(%s) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	good := mustTime(t, "2026-04-06T09:00:00-07:00")
	if err := Validate(good, good.Add(30*time.Minute)); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	cases := []struct {
		name    string
		start   string
		end     string
		wantErr error
	}{
		{
			name:    "60 min duration",
			start:   "2026-04-06T09:00:00-07:00",
			end:     "2026-04-06T10:00:00-07:00",
			wantErr: ErrWrongDuration,
		},
		{
			name:    "starts at :15",
			start:   "2026-04-06T09:15:00-07:00",
			end:     "2026-04-06T09:45:00-07:00",
			wantErr: ErrNotHalfHour,
		},
		{
			name:    "saturday",
			start:   "2026-04-04T09:00:00-07:00",
			end:     "2026-04-04T09:30:00-07:00",
			wantErr: ErrOutsideBusinessHours,
		},
		{
			name:    "5pm boundary",
			start:   "2026-04-06T17:00:00-07:00",
			end:     "2026-04-06T17:30:00-07:00",
			wantErr: ErrOutsideBusinessHours,
		},
		{
			name:    "inverted window",
			start:   "2026-04-06T10:00:00-07:00",
			end:     "2026-04-06T09:30:00-07:00",
			wantErr: ErrInvalidWindow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(mustTime(t, tc.start), mustTime(t, tc.end))
			if err != tc.wantErr {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestGenerateCandidateSlots_FullDay(t *testing.T) {
	// Monday in Pacific time, 8am to 5pm = 18 slots.
	start := mustTime(t, "2026-04-06T08:00:00-07:00")
	end := mustTime(t, "2026-04-06T17:00:00-07:00")
	slots := GenerateCandidateSlots(start, end)
	if len(slots) != 18 {
		t.Fatalf("expected 18 slots, got %d", len(slots))
	}
	if !slots[0].StartsAt.Equal(start) {
		t.Fatalf("first slot = %v, want %v", slots[0].StartsAt, start)
	}
	lastStart := mustTime(t, "2026-04-06T16:30:00-07:00")
	if !slots[len(slots)-1].StartsAt.Equal(lastStart) {
		t.Fatalf("last slot = %v, want %v", slots[len(slots)-1].StartsAt, lastStart)
	}
}

func TestGenerateCandidateSlots_SkipsWeekends(t *testing.T) {
	// Friday 4pm through Monday 9am.
	start := mustTime(t, "2026-04-03T16:00:00-07:00")
	end := mustTime(t, "2026-04-06T09:00:00-07:00")
	slots := GenerateCandidateSlots(start, end)
	// Friday: 16:00, 16:30 (2). Monday: 08:00, 08:30 (2). = 4 slots.
	if len(slots) != 4 {
		t.Fatalf("expected 4 slots across weekend, got %d", len(slots))
	}
	for _, s := range slots {
		switch s.StartsAt.In(PacificLocation()).Weekday() {
		case time.Saturday, time.Sunday:
			t.Fatalf("got weekend slot: %v", s.StartsAt)
		}
	}
}

func TestGenerateCandidateSlots_AlignsUp(t *testing.T) {
	start := mustTime(t, "2026-04-06T08:07:00-07:00") // pre-:30
	end := mustTime(t, "2026-04-06T10:00:00-07:00")
	slots := GenerateCandidateSlots(start, end)
	if len(slots) == 0 {
		t.Fatal("expected slots")
	}
	first := slots[0].StartsAt.In(PacificLocation())
	if first.Hour() != 8 || first.Minute() != 30 {
		t.Fatalf("first slot should be 08:30, got %v:%v", first.Hour(), first.Minute())
	}
}

func TestFilterAvailable(t *testing.T) {
	candidates := GenerateCandidateSlots(
		mustTime(t, "2026-04-06T08:00:00-07:00"),
		mustTime(t, "2026-04-06T10:00:00-07:00"),
	)
	if len(candidates) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(candidates))
	}
	booked := []Appointment{
		{
			TrainerID: 1,
			UserID:    99,
			StartsAt:  mustTime(t, "2026-04-06T09:00:00-07:00"),
			EndsAt:    mustTime(t, "2026-04-06T09:30:00-07:00"),
		},
	}
	available := FilterAvailable(candidates, booked)
	if len(available) != 3 {
		t.Fatalf("expected 3 available, got %d", len(available))
	}
	for _, s := range available {
		if s.StartsAt.Equal(booked[0].StartsAt) {
			t.Fatal("booked slot leaked into availability")
		}
	}
}

func TestFilterAvailable_BookingInDifferentTimezoneStillMatches(t *testing.T) {
	// Same instant, expressed once in Pacific and once in UTC.
	candidates := []Slot{
		{
			StartsAt: mustTime(t, "2026-04-06T09:00:00-07:00"),
			EndsAt:   mustTime(t, "2026-04-06T09:30:00-07:00"),
		},
	}
	booked := []Appointment{
		{
			StartsAt: mustTime(t, "2026-04-06T16:00:00Z"),
			EndsAt:   mustTime(t, "2026-04-06T16:30:00Z"),
		},
	}
	available := FilterAvailable(candidates, booked)
	if len(available) != 0 {
		t.Fatalf("expected slot to be filtered, got %d", len(available))
	}
}

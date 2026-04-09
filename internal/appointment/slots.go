package appointment

import (
	"errors"
	"time"
)

// SlotDuration is the fixed length of every appointment.
const SlotDuration = 30 * time.Minute

// Business hours: M-F, 8:00am - 5:00pm Pacific. The last valid slot starts at
// 4:30pm and ends at 5:00pm.
const (
	businessStartHour = 8
	businessEndHour   = 17 // exclusive end-of-day boundary (5:00pm)
)

var (
	// ErrInvalidWindow is returned when start/end times don't bound a valid window.
	ErrInvalidWindow = errors.New("starts_at must be before ends_at")
	// ErrNotHalfHour is returned when an appointment doesn't fall on :00 or :30.
	ErrNotHalfHour = errors.New("appointment must start on :00 or :30")
	// ErrWrongDuration is returned when an appointment isn't exactly 30 minutes.
	ErrWrongDuration = errors.New("appointment must be exactly 30 minutes")
	// ErrOutsideBusinessHours is returned when an appointment falls outside M-F 8a-5p PT.
	ErrOutsideBusinessHours = errors.New("appointment is outside business hours")
)

// PacificLocation returns the IANA Pacific timezone (handles DST).
func PacificLocation() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		// Fallback to a fixed offset if tzdata is missing — Go embeds tzdata in
		// most builds, but be defensive in stripped containers.
		return time.FixedZone("PST", -8*3600)
	}
	return loc
}

// IsValidSlotStart reports whether t is a valid appointment start: aligned to
// :00 or :30, on a weekday, and within 8:00–16:30 Pacific local time.
func IsValidSlotStart(t time.Time) bool {
	pt := t.In(PacificLocation())

	if pt.Second() != 0 || pt.Nanosecond() != 0 {
		return false
	}
	if pt.Minute() != 0 && pt.Minute() != 30 {
		return false
	}
	switch pt.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	hour := pt.Hour()
	if hour < businessStartHour || hour >= businessEndHour {
		return false
	}
	return true
}

// Validate returns nil if the (startsAt, endsAt) pair represents a well-formed
// 30-minute appointment inside business hours.
func Validate(startsAt, endsAt time.Time) error {
	if !startsAt.Before(endsAt) {
		return ErrInvalidWindow
	}
	if endsAt.Sub(startsAt) != SlotDuration {
		return ErrWrongDuration
	}
	if !IsValidSlotStart(startsAt) {
		pt := startsAt.In(PacificLocation())
		if pt.Minute() != 0 && pt.Minute() != 30 {
			return ErrNotHalfHour
		}
		if pt.Second() != 0 || pt.Nanosecond() != 0 {
			return ErrNotHalfHour
		}
		return ErrOutsideBusinessHours
	}
	return nil
}

// GenerateCandidateSlots returns every valid 30-minute slot whose start time
// falls within [startsAt, endsAt). Times are returned in Pacific time.
func GenerateCandidateSlots(startsAt, endsAt time.Time) []Slot {
	if !startsAt.Before(endsAt) {
		return nil
	}
	loc := PacificLocation()
	cur := alignUpToHalfHour(startsAt.In(loc))
	end := endsAt.In(loc)

	slots := make([]Slot, 0)
	for cur.Before(end) {
		if IsValidSlotStart(cur) {
			slots = append(slots, Slot{
				StartsAt: cur,
				EndsAt:   cur.Add(SlotDuration),
			})
		}
		cur = cur.Add(SlotDuration)
	}
	return slots
}

// alignUpToHalfHour rounds t up to the next :00 or :30 boundary.
func alignUpToHalfHour(t time.Time) time.Time {
	// Strip sub-minute precision first.
	t = t.Truncate(time.Minute)
	min := t.Minute()
	switch {
	case min == 0 || min == 30:
		return t
	case min < 30:
		return t.Add(time.Duration(30-min) * time.Minute)
	default:
		return t.Add(time.Duration(60-min) * time.Minute)
	}
}

// FilterAvailable removes any candidate slot that overlaps an existing
// appointment. Existing appointments are assumed to belong to the same trainer.
func FilterAvailable(candidates []Slot, existing []Appointment) []Slot {
	if len(existing) == 0 {
		return candidates
	}
	taken := make(map[time.Time]struct{}, len(existing))
	for _, a := range existing {
		taken[a.StartsAt.UTC()] = struct{}{}
	}
	out := make([]Slot, 0, len(candidates))
	for _, s := range candidates {
		if _, found := taken[s.StartsAt.UTC()]; found {
			continue
		}
		out = append(out, s)
	}
	return out
}

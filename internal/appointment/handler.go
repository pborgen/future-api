package appointment

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/pborgen/future-api/internal/httputil"
)

// Handler exposes the appointment HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler constructs the handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Routes registers the appointment endpoints onto a gin router.
func (h *Handler) Routes(r gin.IRouter) {
	r.GET("/trainers/:trainer_id/availability", h.getAvailability)
	r.GET("/trainers/:trainer_id/appointments", h.listAppointments)
	r.POST("/appointments", h.createAppointment)
}

// getAvailability godoc
// @Summary      List available appointment slots for a trainer
// @Description  Returns every bookable 30-minute slot for the trainer between starts_at and ends_at. Slots that overlap an existing booking are excluded. Times must be RFC3339.
// @Tags         appointments
// @Produce      json
// @Param        trainer_id  path      int     true  "Trainer ID"
// @Param        starts_at   query     string  true  "Window start (RFC3339)"  example(2026-04-06T08:00:00-07:00)
// @Param        ends_at     query     string  true  "Window end (RFC3339)"    example(2026-04-06T17:00:00-07:00)
// @Success      200  {array}   appointment.Slot
// @Failure      400  {object}  httputil.ErrorResponse
// @Failure      500  {object}  httputil.ErrorResponse
// @Router       /trainers/{trainer_id}/availability [get]
func (h *Handler) getAvailability(c *gin.Context) {
	trainerID, err := parseTrainerID(c)
	if err != nil {
		httputil.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	startsAt, err := httputil.ParseTimeQuery(c, "starts_at")
	if err != nil {
		httputil.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	endsAt, err := httputil.ParseTimeQuery(c, "ends_at")
	if err != nil {
		httputil.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}

	slots, err := h.svc.Available(c.Request.Context(), trainerID, startsAt, endsAt)
	if err != nil {
		httputil.WriteError(c, http.StatusInternalServerError, "failed to load availability")
		return
	}
	c.JSON(http.StatusOK, slots)
}

// listAppointments godoc
// @Summary      List a trainer's scheduled appointments
// @Description  Returns every appointment booked with the given trainer, ordered by start time.
// @Tags         appointments
// @Produce      json
// @Param        trainer_id  path      int  true  "Trainer ID"
// @Success      200  {array}   appointment.Appointment
// @Failure      400  {object}  httputil.ErrorResponse
// @Failure      500  {object}  httputil.ErrorResponse
// @Router       /trainers/{trainer_id}/appointments [get]
func (h *Handler) listAppointments(c *gin.Context) {
	trainerID, err := parseTrainerID(c)
	if err != nil {
		httputil.WriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	apps, err := h.svc.ListForTrainer(c.Request.Context(), trainerID)
	if err != nil {
		httputil.WriteError(c, http.StatusInternalServerError, "failed to load appointments")
		return
	}
	c.JSON(http.StatusOK, apps)
}

// createAppointment godoc
// @Summary      Book a new appointment
// @Description  Creates a 30-minute appointment for the given trainer and user. The slot must start on :00 or :30, fall within M-F 8am-5pm Pacific, and not overlap an existing booking for the trainer.
// @Tags         appointments
// @Accept       json
// @Produce      json
// @Param        request  body      appointment.CreateRequest  true  "Appointment to create"
// @Success      201  {object}  appointment.Appointment
// @Failure      400  {object}  httputil.ErrorResponse  "malformed request"
// @Failure      409  {object}  httputil.ErrorResponse  "trainer already booked at that time"
// @Failure      422  {object}  httputil.ErrorResponse  "outside business hours, wrong duration, or not on :00/:30"
// @Failure      500  {object}  httputil.ErrorResponse
// @Router       /appointments [post]
func (h *Handler) createAppointment(c *gin.Context) {
	var req CreateRequest
	dec := newStrictJSONDecoder(c)
	if err := dec.Decode(&req); err != nil {
		httputil.WriteError(c, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.TrainerID == 0 || req.UserID == 0 {
		httputil.WriteError(c, http.StatusBadRequest, "trainer_id and user_id are required")
		return
	}

	created, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrConflict):
			httputil.WriteError(c, http.StatusConflict, err.Error())
		case errors.Is(err, ErrInvalidWindow),
			errors.Is(err, ErrNotHalfHour),
			errors.Is(err, ErrWrongDuration),
			errors.Is(err, ErrOutsideBusinessHours):
			httputil.WriteError(c, http.StatusUnprocessableEntity, err.Error())
		default:
			httputil.WriteError(c, http.StatusInternalServerError, "failed to create appointment")
		}
		return
	}
	c.JSON(http.StatusCreated, created)
}

func parseTrainerID(c *gin.Context) (int64, error) {
	raw := c.Param("trainer_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid trainer_id")
	}
	return id, nil
}

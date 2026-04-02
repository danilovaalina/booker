package api

import (
	"context"
	"net/http"
	"time"

	"booker/internal/model"
	"booker/internal/service"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Service interface {
	CreateEvent(ctx context.Context, opts service.CreateEventOptions) (model.Event, error)
	Event(ctx context.Context, id uuid.UUID) (model.Event, error)
	Events(ctx context.Context) ([]model.Event, error)
	BookSlot(ctx context.Context, opts service.BookSlotOptions) (model.Booking, error)
	ConfirmBooking(ctx context.Context, opts service.ConfirmBookingOptions) error
}

type API struct {
	*echo.Echo
	service Service
}

func New(service Service) *API {
	a := &API{
		Echo:    echo.New(),
		service: service,
	}

	// Раздача статичных файлов из папки web
	a.Static("/static", "web")
	a.File("/admin", "web/admin.html")
	a.File("/", "web/user.html")

	a.POST("/events", a.createEvent)
	a.GET("/events/:id", a.event)
	a.GET("/events", a.events)
	a.POST("/events/:id/book", a.bookSlot)
	a.POST("/bookings/:id/confirm", a.confirmBooking)

	return a
}

type createEventRequest struct {
	Title          string `json:"title"`
	TotalSlots     int    `json:"total_slots"`
	TimeoutMinutes int    `json:"timeout_minutes"`
}

type eventResponse struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	TotalSlots     int       `json:"total_slots"`
	BookedSlots    int       `json:"booked_slots"`
	TimeoutMinutes int       `json:"timeout_minutes"`
}

func eventToResponse(e model.Event) eventResponse {
	return eventResponse{
		ID:             e.ID,
		Title:          e.Title,
		TotalSlots:     e.TotalSlots,
		BookedSlots:    e.BookedSlots,
		TimeoutMinutes: e.TimeoutMinutes,
	}
}

func (a *API) createEvent(c echo.Context) error {
	var req createEventRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	event, err := a.service.CreateEvent(c.Request().Context(), service.CreateEventOptions{
		Title:          req.Title,
		TotalSlots:     req.TotalSlots,
		TimeoutMinutes: req.TimeoutMinutes,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, eventToResponse(event))
}

func (a *API) event(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid uuid"})
	}

	event, err := a.service.Event(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "event not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal error"})
	}

	return c.JSON(http.StatusOK, eventToResponse(event))
}

func (a *API) events(c echo.Context) error {
	events, err := a.service.Events(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to fetch events"})
	}

	resp := make([]eventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, eventToResponse(e))
	}

	return c.JSON(http.StatusOK, resp)
}

type bookingResponse struct {
	ID      uuid.UUID `json:"id"`
	EventID uuid.UUID `json:"event_id"`
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
	Expires time.Time `json:"expires"`
}

func bookingToResponse(b model.Booking) bookingResponse {
	return bookingResponse{
		ID:      b.ID,
		EventID: b.Event.ID,
		Status:  string(b.Status),
		Created: b.Created,
		Expires: b.Expires,
	}
}

type bookRequest struct {
	EventID   uuid.UUID `param:"id"`
	Recipient string    `json:"recipient"`
	Channel   string    `json:"channel"`
}

func (a *API) bookSlot(c echo.Context) error {
	var req bookRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	booking, err := a.service.BookSlot(c.Request().Context(), service.BookSlotOptions{
		EventID:   req.EventID,
		Recipient: req.Recipient,
		Channel:   model.NotificationChannel(req.Channel),
	})
	if err != nil {
		if errors.Is(err, model.ErrNoSlots) {
			return c.JSON(http.StatusConflict, echo.Map{"error": "no free slots available for this event"})
		}
		if errors.Is(err, model.ErrNotFound) {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "event not found"})
		}
		if errors.Is(err, model.ErrUnsupportedChannel) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "unsupported notification channel"})
		}

		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to book slot"})
	}

	return c.JSON(http.StatusCreated, bookingToResponse(booking))
}

type confirmRequest struct {
	ID        uuid.UUID `param:"id"`
	PaymentID string    `json:"payment_id"`
}

func (a *API) confirmBooking(c echo.Context) error {
	var req confirmRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request format or uuid"})
	}

	err := a.service.ConfirmBooking(c.Request().Context(), service.ConfirmBookingOptions{
		BookingID: req.ID,
		PaymentID: req.PaymentID,
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return c.JSON(http.StatusGone, echo.Map{"error": "booking expired or already confirmed"})
		}

		if errors.Is(err, model.ErrPaymentRequired) || errors.Is(err, model.ErrInvalidPayment) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}

		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to confirm booking"})
	}

	return c.NoContent(http.StatusNoContent)
}

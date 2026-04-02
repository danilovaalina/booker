package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"booker/internal/model"
	"booker/internal/repository"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	msgBookingCancelled = "⚠️ Ваша бронь на мероприятие <b>%s</b> отменена из-за отсутствия оплаты."
)

type Repository interface {
	CreateEvent(ctx context.Context, opts repository.CreateEventOptions) (model.Event, error)
	Event(ctx context.Context, id uuid.UUID) (model.Event, error)
	Events(ctx context.Context) ([]model.Event, error)
	BookSlot(ctx context.Context, opts repository.CreateBookingOptions) (model.Booking, error)
	CancelExpiredBookings(ctx context.Context) ([]model.Booking, error)
	ConfirmBooking(ctx context.Context, opts repository.ConfirmBookingOptions) error
}

type Sender interface {
	Send(ctx context.Context, n model.Notification) error
}

type Service struct {
	repo           Repository
	sender         Sender
	defaultTimeout int
}

type Options struct {
	Repo           Repository
	Sender         Sender
	DefaultTimeout int
}

func New(opts Options) *Service {
	return &Service{repo: opts.Repo, sender: opts.Sender, defaultTimeout: opts.DefaultTimeout}
}

type CreateEventOptions struct {
	Title          string
	TotalSlots     int
	TimeoutMinutes int
}

func (s *Service) CreateEvent(ctx context.Context, opts CreateEventOptions) (model.Event, error) {
	if opts.Title == "" {
		return model.Event{}, errors.New("название мероприятия не может быть пустым")
	}
	if opts.TotalSlots <= 0 {
		return model.Event{}, errors.New("количество мест должно быть больше нуля")
	}

	timeout := opts.TimeoutMinutes
	if timeout <= 0 {
		timeout = s.defaultTimeout
	}

	event, err := s.repo.CreateEvent(ctx, repository.CreateEventOptions{
		ID:             uuid.New(),
		Title:          opts.Title,
		TotalSlots:     opts.TotalSlots,
		TimeoutMinutes: timeout,
		Created:        time.Now(),
	})
	if err != nil {
		return model.Event{}, err
	}

	return event, nil
}

func (s *Service) Event(ctx context.Context, id uuid.UUID) (model.Event, error) {
	return s.repo.Event(ctx, id)
}

func (s *Service) Events(ctx context.Context) ([]model.Event, error) {
	return s.repo.Events(ctx)
}

type BookSlotOptions struct {
	EventID   uuid.UUID
	Recipient string
	Channel   model.NotificationChannel
}

func (s *Service) BookSlot(ctx context.Context, opts BookSlotOptions) (model.Booking, error) {
	if opts.EventID == uuid.Nil {
		return model.Booking{}, model.ErrEventRequired
	}

	if opts.Channel != model.ChannelNone {
		if opts.Channel != model.ChannelEmail && opts.Channel != model.ChannelTelegram {
			return model.Booking{}, model.ErrUnsupportedChannel
		}
	}

	event, err := s.repo.Event(ctx, opts.EventID)
	if err != nil {
		return model.Booking{}, err
	}

	return s.repo.BookSlot(ctx, repository.CreateBookingOptions{
		ID:        uuid.New(),
		EventID:   event.ID,
		Status:    model.StatusPending,
		Recipient: opts.Recipient,
		Channel:   opts.Channel,
		Expires:   time.Now().Add(time.Duration(event.TimeoutMinutes) * time.Minute),
		Created:   time.Now(),
	})
}

type ConfirmBookingOptions struct {
	BookingID uuid.UUID
	PaymentID string
}

func (s *Service) ConfirmBooking(ctx context.Context, opts ConfirmBookingOptions) error {
	if opts.PaymentID == "" {
		return model.ErrPaymentRequired
	}

	if !strings.HasPrefix(opts.PaymentID, "TXN-") {
		return model.ErrInvalidPayment
	}

	return s.repo.ConfirmBooking(ctx, repository.ConfirmBookingOptions{
		ID:        opts.BookingID,
		PaymentID: opts.PaymentID,
	})
}

func (s *Service) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Info().Dur("interval", interval).Msg("background cleanup started")

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("background cleanup stopped")
				return
			case <-ticker.C:
				cancelled, err := s.repo.CancelExpiredBookings(ctx)
				if err != nil {
					log.Error().Err(err).Msg("cleanup failed")
					continue
				}

				if len(cancelled) > 0 {
					log.Info().Int("count", len(cancelled)).Msg("cleanup: bookings cancelled")

					// Рассылаем уведомления каждому пользователю
					for _, b := range cancelled {
						if b.Recipient == "" || b.Channel == "" {
							continue
						}

						err = s.sender.Send(ctx, model.Notification{
							Channel:   b.Channel,
							Recipient: b.Recipient,
							Message:   fmt.Sprintf(msgBookingCancelled, b.Event.Title),
						})

						if err != nil {
							log.Warn().Err(err).Str("to", b.Recipient).Msg("failed to notify user")
						}
					}
				}
			}
		}
	}()
}

package model

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type BookingStatus string

const (
	StatusPending   BookingStatus = "pending"
	StatusConfirmed BookingStatus = "confirmed"
	StatusCancelled BookingStatus = "cancelled"
)

type Event struct {
	ID             uuid.UUID
	Title          string
	TotalSlots     int
	BookedSlots    int
	TimeoutMinutes int
	Created        time.Time
}

type Booking struct {
	ID        uuid.UUID
	Event     Event
	PaymentID string
	Recipient string
	Channel   NotificationChannel
	Status    BookingStatus
	Created   time.Time
	Expires   time.Time
}

type NotificationChannel string

const (
	ChannelTelegram NotificationChannel = "telegram"
	ChannelEmail    NotificationChannel = "email"
	ChannelNone     NotificationChannel = ""
)

type Notification struct {
	Channel   NotificationChannel
	Recipient string
	Message   string
}

var (
	ErrNoSlots            = errors.New("no free slots available")
	ErrNotFound           = errors.New("event not found")
	ErrPaymentRequired    = errors.New("payment_id is required")
	ErrEventRequired      = errors.New("event_id is required")
	ErrInvalidPayment     = errors.New("invalid payment transaction format")
	ErrUnsupportedChannel = errors.New("unsupported notification channel")
)

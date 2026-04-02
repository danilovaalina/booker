package repository

import (
	"context"
	"time"

	"booker/internal/model"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

type CreateEventOptions struct {
	ID             uuid.UUID
	Title          string
	TotalSlots     int
	TimeoutMinutes int
	Created        time.Time
}

func (r *Repository) CreateEvent(ctx context.Context, opts CreateEventOptions) (model.Event, error) {
	query := `
		insert into event (id, title, total_slots, timeout_minutes, created)
		values ($1, $2, $3, $4, $5)
		returning id, title, total_slots, booked_slots, timeout_minutes, created
	`

	rows, err := r.pool.Query(ctx, query, opts.ID, opts.Title, opts.TotalSlots, opts.TimeoutMinutes, opts.Created)
	if err != nil {
		return model.Event{}, errors.WithStack(err)
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow[eventRow](rows, pgx.RowToStructByNameLax[eventRow])
	if err != nil {
		return model.Event{}, errors.WithStack(err)
	}

	return r.eventModel(row), nil
}

func (r *Repository) Event(ctx context.Context, id uuid.UUID) (model.Event, error) {
	query := `
		select id, title, total_slots, booked_slots, timeout_minutes, created 
		from event 
		where id = $1`

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return model.Event{}, errors.WithStack(err)
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow[eventRow](rows, pgx.RowToStructByNameLax[eventRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Event{}, errors.WithStack(model.ErrNotFound)
		}
		return model.Event{}, errors.WithStack(err)
	}

	return r.eventModel(row), nil
}

func (r *Repository) Events(ctx context.Context) ([]model.Event, error) {
	query := `
		select id, title, total_slots, booked_slots, timeout_minutes, created 
		from event 
		order by created desc`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rows.Close()

	eventRows, err := pgx.CollectRows[eventRow](rows, pgx.RowToStructByNameLax[eventRow])
	if err != nil {
		return nil, errors.WithStack(err)
	}

	events := make([]model.Event, 0, len(eventRows))
	for _, row := range eventRows {
		events = append(events, r.eventModel(row))
	}

	return events, nil
}

type CreateBookingOptions struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	Recipient string
	Channel   model.NotificationChannel
	Status    model.BookingStatus
	Expires   time.Time
	Created   time.Time
}

func (r *Repository) BookSlot(ctx context.Context, opts CreateBookingOptions) (model.Booking, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Booking{}, errors.WithStack(err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	res, err := tx.Exec(ctx, `
		update event 
		set booked_slots = booked_slots + 1 
		where id = $1 and booked_slots < total_slots`,
		opts.EventID,
	)
	if err != nil {
		return model.Booking{}, errors.WithStack(err)
	}

	if res.RowsAffected() == 0 {
		return model.Booking{}, model.ErrNoSlots
	}

	query := `
		insert into booking (id, event_id, recipient, channel, status, expires, created) 
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, event_id, recipient, channel, status, created, expires`

	rows, err := tx.Query(ctx, query,
		opts.ID, opts.EventID, opts.Recipient, opts.Channel, opts.Status, opts.Expires, opts.Created)
	if err != nil {
		return model.Booking{}, errors.WithStack(err)
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow[bookingRow](rows, pgx.RowToStructByNameLax[bookingRow])
	if err != nil {
		return model.Booking{}, errors.WithStack(err)
	}

	if err = tx.Commit(ctx); err != nil {
		return model.Booking{}, errors.WithStack(err)
	}

	return r.bookingModel(row), nil
}

func (r *Repository) CancelExpiredBookings(ctx context.Context) ([]model.Booking, error) {
	query := `
		with expired as (
			update booking
			set status = 'cancelled'
			where status = 'pending' and expires < now()
			returning id, event_id, recipient, channel
		),
		sub as (
			select event_id, count(*) as cnt from expired group by event_id
		),
		update_events as (
			update event e
			set booked_slots = e.booked_slots - sub.cnt
			from sub
			where e.id = sub.event_id
		)
		select 
			er.id, er.event_id, ev.title as event_title, er.recipient, er.channel 
		from expired er
		join event ev on er.event_id = ev.id`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rows.Close()

	bookingRows, err := pgx.CollectRows[cancelBookingRow](rows, pgx.RowToStructByNameLax[cancelBookingRow])
	if err != nil {
		return nil, errors.WithStack(err)
	}

	bookings := make([]model.Booking, 0, len(bookingRows))
	for _, row := range bookingRows {
		bookings = append(bookings, r.canceledBookingModel(row))
	}

	return bookings, nil
}

type ConfirmBookingOptions struct {
	ID        uuid.UUID
	PaymentID string
}

func (r *Repository) ConfirmBooking(ctx context.Context, opts ConfirmBookingOptions) error {
	query := `
		update booking
		set status = 'confirmed', payment_id = $2
		where id = $1 and status = 'pending' and expires > now()`

	res, err := r.pool.Exec(ctx, query, opts.ID, opts.PaymentID)
	if err != nil {
		return errors.WithStack(err)
	}

	if res.RowsAffected() == 0 {
		// Либо бронь уже отменена, либо уже подтверждена, либо её нет
		return model.ErrNotFound
	}

	return nil
}

type eventRow struct {
	ID             uuid.UUID `db:"id"`
	Title          string    `db:"title"`
	TotalSlots     int       `db:"total_slots"`
	BookedSlots    int       `db:"booked_slots"`
	TimeoutMinutes int       `db:"timeout_minutes"`
	Created        time.Time `db:"created"`
}

func (r *Repository) eventModel(row eventRow) model.Event {
	return model.Event{
		ID:             row.ID,
		Title:          row.Title,
		TotalSlots:     row.TotalSlots,
		BookedSlots:    row.BookedSlots,
		TimeoutMinutes: row.TimeoutMinutes,
		Created:        row.Created,
	}
}

type bookingRow struct {
	ID        uuid.UUID `db:"id"`
	EventID   uuid.UUID `db:"event_id"`
	Recipient string    `db:"recipient"`
	Channel   string    `db:"channel"`
	Status    string    `db:"status"`
	Created   time.Time `db:"created"`
	Expires   time.Time `db:"expires"`
}

func (r *Repository) bookingModel(row bookingRow) model.Booking {
	return model.Booking{
		ID:        row.ID,
		Event:     model.Event{ID: row.EventID},
		Recipient: row.Recipient,
		Channel:   model.NotificationChannel(row.Channel),
		Status:    model.BookingStatus(row.Status),
		Created:   row.Created,
		Expires:   row.Expires,
	}
}

type cancelBookingRow struct {
	ID         uuid.UUID `db:"id"`
	EventID    uuid.UUID `db:"event_id"`
	Recipient  string    `db:"recipient"`
	Channel    string    `db:"channel"`
	EventTitle string    `db:"event_title"`
}

func (r *Repository) canceledBookingModel(row cancelBookingRow) model.Booking {
	return model.Booking{
		ID:        row.ID,
		Recipient: row.Recipient,
		Channel:   model.NotificationChannel(row.Channel),
		Status:    model.StatusCancelled,
		Event: model.Event{
			ID:    row.EventID,
			Title: row.EventTitle,
		},
	}
}

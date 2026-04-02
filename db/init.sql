-- таблица мероприятий
create table if not exists event
(
    id              uuid primary key         default gen_random_uuid(),
    title           text    not null,
    total_slots     integer not null,
    booked_slots    integer                  default 0,
    timeout_minutes integer not null         default 15,
    created         timestamp with time zone default now()
);

create type booking_status as enum ('pending', 'confirmed', 'cancelled');

-- таблица бронирований
create table if not exists booking
(
    id         uuid primary key                  default gen_random_uuid(),
    event_id   uuid references event (id) on delete cascade,
    status     booking_status           not null default 'pending', -- pending, confirmed, cancelled
    recipient  text, -- Email или ChatID
    channel    text, -- 'email' или 'telegram'
    payment_id text,
    created    timestamp with time zone          default now(),
    expires    timestamp with time zone not null
);

create index if not exists idx_booking_status_expires on booking (status, expires)
    where status = 'pending';
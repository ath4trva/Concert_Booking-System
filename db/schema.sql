-- Database Schema for Concert Booking System
-- Save this file as db.sql

-- 1. Create Events Table First (referenced by bookings)
CREATE TABLE IF NOT EXISTS public.events
(
    id SERIAL PRIMARY KEY,
    name text NOT NULL,
    total_tickets integer NOT NULL,
    available_tickets integer NOT NULL
);

ALTER TABLE IF EXISTS public.events
    OWNER to postgres;


-- 2. Create Bookings Table
CREATE TABLE IF NOT EXISTS public.bookings
(
    id SERIAL PRIMARY KEY,
    event_id integer,
    user_id integer NOT NULL,
    bookings_time timestamp without time zone DEFAULT now(),
    idempotency_key text,
    status text DEFAULT 'confirmed',
    expires_at timestamp without time zone,
    
    -- Constraints
    CONSTRAINT bookings_event_id_user_id_key UNIQUE (event_id, user_id),
    CONSTRAINT bookings_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT bookings_event_id_fkey FOREIGN KEY (event_id)
        REFERENCES public.events (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

ALTER TABLE IF EXISTS public.bookings
    OWNER to postgres;


-- 3. Query Templates (For reference in application code)
-- Note: '?' placeholders work in prepared statements (Java, Python, etc.) 
-- but cannot be run directly in SQL.

/*
INSERT INTO public.bookings(
    event_id, user_id, bookings_time, idempotency_key, status, expires_at)
VALUES (?, ?, ?, ?, ?, ?);
*/
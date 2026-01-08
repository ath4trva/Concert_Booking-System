CREATE TABLE events (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    total_tickets INT NOT NULL,
    available_tickets INT NOT NULL
);

CREATE TABLE bookings(
    id SERIAL PRIMARY KEY,
    event_id INT REFERENCES events(id),
    user_id INT NOT NULL,
    bookings_time TIMESTAMP DEFAULT NOW(),
    UNIQUE(event_id, user_id)
);

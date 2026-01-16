package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not found, using system env")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error opening database: ", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Database connection failed: ", err)
	}
	fmt.Println("--- Successfully connected to PostgreSQL ---")

	ctx := context.Background()

	db.Exec("DELETE FROM bookings")
	db.Exec("DELETE FROM events")

	eventID, err := CreateEvent(ctx, "Go Conference 2026", 5)
	if err != nil {
		log.Fatal("Failed to create event: ", err)
	}
	fmt.Printf("Step 1: Event Created with ID %d\n", eventID)

	fmt.Println("Step 2: Attempting to book ticket for User 101...")
	if err := BookTicket(ctx, eventID, 101); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Success: User 101 booked a ticket!")
	}

	fmt.Println("Step 3: Attempting duplicate booking for User 101...")
	if err := BookTicket(ctx, eventID, 101); err != nil {
		fmt.Printf("Caught Expected Error: %v\n", err)
	}

	fmt.Println("Step 4: Attempting to cancel User 101's booking...")
	if err := CancelBooking(ctx, eventID, 101); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Success: Booking cancelled, ticket returned to pool.")
	}
}

func CreateEvent(ctx context.Context, name string, tickets int) (int, error) {
	var id int
	query := `INSERT INTO events (name, total_tickets, available_tickets) 
              VALUES ($1, $2, $3) RETURNING id`

	err := db.QueryRowContext(ctx, query, name, tickets, tickets).Scan(&id)
	return id, err
}

func BookTicket(ctx context.Context, eventID int, userID int) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists bool
	checkDupQuery := `SELECT EXISTS(SELECT 1 FROM bookings WHERE event_id=$1 AND user_id=$2)`
	err = tx.QueryRowContext(ctx, checkDupQuery, eventID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("user already has a booking for this event")
	}

	var available int
	checkTicketsQuery := `SELECT available_tickets FROM events WHERE id=$1`
	err = tx.QueryRowContext(ctx, checkTicketsQuery, eventID).Scan(&available)
	if err != nil {
		return err
	}
	if available <= 0 {
		return errors.New("insufficient tickets available")
	}

	updateQuery := `UPDATE events SET available_tickets = available_tickets - 1 WHERE id=$1`
	_, err = tx.ExecContext(ctx, updateQuery, eventID)
	if err != nil {
		return err
	}

	insertQuery := `INSERT INTO bookings (event_id, user_id) VALUES ($1, $2)`
	_, err = tx.ExecContext(ctx, insertQuery, eventID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func CancelBooking(ctx context.Context, eventID int, userID int) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, "DELETE FROM bookings WHERE event_id=$1 AND user_id=$2", eventID, userID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("no booking found to cancel")
	}

	_, err = tx.ExecContext(ctx, "UPDATE events SET available_tickets = available_tickets + 1 WHERE id=$1", eventID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

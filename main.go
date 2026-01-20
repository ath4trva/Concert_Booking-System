package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
)

var db *sql.DB

var (
	bookingsSuccess int64
	bookingsFailed  int64
	retryCount      int64
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Warn("Note: .env file not found.")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("Error opening database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	resetDatabase()
	totalTickets := 10
	eventID, err := CreateEvent(context.Background(), "Pepsi Super Bowl Halftime", totalTickets)
	if err != nil {
		slog.Error("Failed to create event", "error", err)
		os.Exit(1)
	}

	slog.Info("STARTING ROBUST STRESS TEST (Week 4)", "tickets", totalTickets, "users", 100)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	start := time.Now()

	go func() {
		runStressTest(eventID, 100)
		stopChan <- os.Interrupt
	}()

	<-stopChan
	slog.Info("Shutting down... (Test Finished or Interrupted)")

	duration := time.Since(start)

	verifyResults(eventID, totalTickets)
	fmt.Println("\n---------------------------------------------------")
	fmt.Printf("Execution Time: %v\n", duration)
	fmt.Printf("Successful Bookings: %d\n", bookingsSuccess)
	fmt.Printf("Failed Bookings:     %d\n", bookingsFailed)
	fmt.Printf("Retries (Conflicts): %d\n", retryCount)
	fmt.Println("---------------------------------------------------")
}

func runStressTest(eventID int, concurrentUser int) {
	var wg sync.WaitGroup

	for i := 0; i < concurrentUser; i++ {
		wg.Add(1)
		userID := 1000 + i

		go func(uID int) {
			defer wg.Done()

			idempotencyKey := fmt.Sprintf("event-%d-user-%d", eventID, uID)

			maxRetries := 5
			for attempt := 0; attempt <= maxRetries; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

				err := BookTicketIdempotent(ctx, eventID, uID, idempotencyKey)
				cancel()

				if err == nil {
					atomic.AddInt64(&bookingsSuccess, 1)
					slog.Info("GOT TICKET!", "user_id", uID)
					return
				}

				if isSerializationFailure(err) {
					atomic.AddInt64(&retryCount, 1)
					time.Sleep(time.Duration(rand.Intn(50)+10) * time.Millisecond)
					continue
				}

				atomic.AddInt64(&bookingsFailed, 1)
				slog.Info("SOLD OUT", "user_id", uID)
				return
			}

			slog.Error("GAVE UP", "user_id", uID)
			atomic.AddInt64(&bookingsFailed, 1)
		}(userID)
	}
	wg.Wait()
}

func BookTicketIdempotent(ctx context.Context, eventID int, userID int, idenKey string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingID int
	err = tx.QueryRowContext(ctx, "SELECT id FROM bookings WHERE idempotency_key=$1", idenKey).Scan(&existingID)

	if err == nil {
		slog.Info("Idempotency Triggered: Request already processed", "user_id", userID)
		return nil
	}

	var available int
	err = tx.QueryRowContext(ctx, "SELECT available_tickets FROM events WHERE id=$1", eventID).Scan(&available)
	if err != nil {
		return err
	}

	if available <= 0 {
		return errors.New("sold out")
	}

	_, err = tx.ExecContext(ctx, "UPDATE events SET available_tickets = available_tickets - 1 where id=$1", eventID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO bookings (event_id, user_id, idempotency_key) VALUES ($1, $2, $3)",
		eventID, userID, idenKey)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func isSerializationFailure(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "40001"
	}
	return false
}

func verifyResults(eventID int, expectedLimit int) {
	var finalCount int
	db.QueryRow("SELECT COUNT(*) FROM bookings WHERE event_id=$1", eventID).Scan(&finalCount)
	fmt.Printf("\n--- VERIFICATION ---\nExpected: %d\nActual:   %d\n", expectedLimit, finalCount)
}

func CreateEvent(ctx context.Context, name string, tickets int) (int, error) {
	var id int
	err := db.QueryRowContext(ctx, "INSERT INTO events (name, total_tickets, available_tickets) VALUES ($1, $2, $3) RETURNING id", name, tickets, tickets).Scan(&id)
	return id, err
}

func resetDatabase() {
	db.Exec("DELETE FROM bookings")
	db.Exec("DELETE FROM events")
}

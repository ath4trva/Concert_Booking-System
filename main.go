package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/tools/go/analysis/passes/defers"
	// Postgres driver
)

var db *sql.DB
var (
	bookingsSuccess int64
	bookingsFailed  int64
	retryCount     int64
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.Default(logger)

	if err := godotenv.Load(); err != nil {
		slog.Warn("Note: .env file not found.")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))

	var err error
	db, err= sql.Open("postgress",connStr)
	if err!=nil{
		slog.Error("Error opening database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	resetDatabase()
	totalTickets:=10
	eventID, err:= CreateEvent(context.Background(),"Pepsi Super Bowl Halftime",totalTickets)
	if err!=nil{
		slog.Error("Failed to create event", "error", err)
	}

	slog.Info("STARTING STRESS TEST", "TICKETS", totalTickets, "USER", 100)

	start:= time.Now()
	runStressTest(eventID,100)
	duration:= time.Since(start)

	verifyResults(eventID, totalTickets)
	fmt.Println("\n---------------------------------------------------")
	fmt.Printf(" Execution Time: %v\n", duration)
	fmt.Printf(" Successful Bookings: %d\n", bookingsSuccess)
	fmt.Printf(" Failed Bookings:     %d\n", bookingsFailed)
	fmt.Printf(" Retries (Conflicts): %d\n", retryCount)
	fmt.Println("---------------------------------------------------")

}

func runStressTest(eventID int, concurrentUser int){
	var wg sync.WaitGroup

	for i:=0; i<=concurrentUser;i++{
		wg.Add(1)
		userID:=1000+i

	go func(uID int){
		defer wg.Done()
		maxRetries:=5
		for attempt:=0; attempt<=maxRetries;attempt++{
			ctx, cancel:= context.WithTimeout(context.Background(),2*time.Second)
			err:= BookingTickets(ctx, eventID,uID)
			cancel()

			if err ==nil{
				atomic.AddInt64(&bookingsSuccess,1)
				slog.Info("Got Ticket!", "user_id", uID)
				return
			}

			if isSerializationFailure(err){
				atomic.AddInt64(&retryCount,1)
				time.Sleep(time.Duration(rand.Intn(50)+10) * time.Millisecond)
				continue
			}

			atomic.AddInt64(&bookingsFailed,1)
			slog.Info("SOLD OUT", "user_id", uID)
			return
		}
		
		slog.Error("GAVE UP (Too many retries)", "user_id", uID)
			atomic.AddInt64(&bookingsFailed, 1)
		}(userID)
	}
	wg.wait()
}
# System Design – Ticket Booking System

## Overview

This project is a backend ticket booking system designed to safely handle
concurrent ticket bookings without overselling. The primary focus of the
design is correctness, data integrity, and predictable behavior under
concurrent access.

PostgreSQL is used as the single source of truth, and all critical operations
are performed inside database transactions.

---

## Problems Addressed by This Design

### 1. Overselling Tickets

**Problem:**  
When multiple users try to book tickets at the same time, the system may sell
more tickets than are actually available.

**Design Solution:**  
Ticket availability is checked and updated inside a database transaction using
row-level locking. This ensures that only one booking operation can modify the
ticket count at a time.

---

### 2. Race Conditions During Concurrent Bookings

**Problem:**  
Two or more booking requests may read the same ticket count simultaneously and
both succeed, leading to inconsistent data.

**Design Solution:**  
The event row is locked during booking so concurrent requests are forced to
wait. Each request sees the latest committed ticket count before proceeding.

---

### 3. Inconsistent Data on Partial Failure

**Problem:**  
If ticket count is reduced but order creation fails, the system may end up in
an invalid state.

**Design Solution:**  
All booking steps (ticket validation, ticket update, and order creation) are
executed within a single database transaction. If any step fails, the entire
operation is rolled back.

---

### 4. Duplicate or Conflicting Booking Attempts

**Problem:**  
Users may accidentally submit the same booking request multiple times,
resulting in duplicate orders or incorrect ticket counts.

**Design Solution:**  
The system is designed to support idempotent booking operations and enforces
consistency at the database level.

---

### 5. Incorrect Ticket Count Under High Load

**Problem:**  
Application-level counters or in-memory state can become inaccurate under
high concurrency or server restarts.

**Design Solution:**  
Ticket counts are stored and updated only in PostgreSQL, which acts as the
single source of truth.

---

### 6. Partial Updates Across Multiple Tables

**Problem:**  
Updating tickets and orders separately can cause mismatches if one update
succeeds and the other fails.

**Design Solution:**  
Both ticket updates and order creation occur within the same transaction,
ensuring atomic updates across tables.

---

### 7. Unsafe Concurrency Handling at Application Level

**Problem:**  
Handling concurrency in application code does not scale safely when multiple
instances of the service are running.

**Design Solution:**  
Concurrency control is delegated to the database, ensuring correctness
regardless of the number of application instances.

---

### 8. Data Corruption During Unexpected Crashes

**Problem:**  
Server crashes during a booking operation may leave the system in an
inconsistent state.

**Design Solution:**  
PostgreSQL transactions guarantee atomicity, ensuring that either all changes
are committed or none are applied.

---

### 9. Non-Deterministic Booking Behavior

**Problem:**  
Without controlled access, booking behavior under concurrency can become
unpredictable and difficult to reason about.

**Design Solution:**  
Row-level locking enforces a clear and deterministic order of ticket allocation
per event.

---

### 10. Premature Optimization Over Correctness

**Problem:**  
Introducing caching or distributed systems early can increase complexity and
introduce subtle consistency bugs.

**Design Solution:**  
This design prioritizes correctness and data integrity first, with performance
optimizations deferred to later stages.

---

## Design Principles

- Database is the single source of truth
- Correctness over performance
- Explicit transactions for all critical operations
- Simple, predictable, and debuggable behavior

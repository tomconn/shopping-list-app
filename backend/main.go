package main

import (
	"context"
	"encoding/json"
	"errors" // Keep for sql.ErrNoRows check if needed, though pgx might have its own ErrNoRows
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"        // Needed for DBPool interface method signatures
	"github.com/jackc/pgx/v5/pgconn" // Needed for DBPool interface method signatures
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv" // Optional: For local .env loading
)

// --- Configuration ---

// DBConfig holds database connection parameters
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Item represents a shopping list item
type Item struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Quantity  string    `json:"quantity"`
	CreatedAt time.Time `json:"created_at,omitempty"` // omitempty for POST
}

// --- Interface for DB Operations ---

// DBPool defines the interface for database operations we need,
// allowing both real pgxpool.Pool and mocks to be used.
type DBPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Ping(ctx context.Context) error
	Close() // Required for graceful shutdown and test cleanup
}

// --- Global Variables ---
// Use the interface type for the global variable
var dbpool DBPool

// --- Database Functions ---

// connectDB initializes the database connection pool
// It still returns the concrete type *pgxpool.Pool, which implements DBPool
func connectDB(cfg DBConfig) (*pgxpool.Pool, error) {
	connString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=10",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string config: %w", err)
	}

	// Recommended settings for robustness
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test the connection
	err = pool.Ping(context.Background())
	if err != nil {
		pool.Close() // Close pool if ping fails
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL database!")
	return pool, nil
}

// createSchemaIfNotExists checks for the items table and creates it if it doesn't exist
// Accepts the DBPool interface type
func createSchemaIfNotExists(pool DBPool) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS items (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL CHECK (name <> ''),
		quantity TEXT NOT NULL CHECK (quantity <> ''),
		created_at TIMESTAMPTZ DEFAULT NOW()
	);`

	_, err := pool.Exec(context.Background(), createTableSQL)
	if err != nil {
		return fmt.Errorf("error creating table schema: %w", err)
	}
	log.Println("Ensured 'items' table exists.")
	return nil
}

// getItems retrieves all items from the database
// Uses the global dbpool (which is of type DBPool)
func getItems(ctx context.Context) ([]Item, error) {
	rows, err := dbpool.Query(ctx, "SELECT id, name, quantity, created_at FROM items ORDER BY created_at DESC")
	if err != nil {
		// Check specifically for pgx's no rows error if necessary, otherwise treat as general DB error
		if errors.Is(err, pgx.ErrNoRows) {
			return []Item{}, nil // Return empty slice for no rows, not an error
		}
		log.Printf("Error querying items: %v\n", err)
		return nil, fmt.Errorf("database query error: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	// Use pgx's CollectRows or Next/Scan loop
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Quantity, &item.CreatedAt); err != nil {
			log.Printf("Error scanning item row: %v\n", err)
			// Continue processing other rows if one fails to scan
			continue
		}
		items = append(items, item)
	}

	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Printf("Error after iterating rows: %v\n", err)
		// It's often better to return the items successfully scanned along with the iteration error
		// But for simplicity here, we return an error indicating partial results might be lost.
		return nil, fmt.Errorf("database iteration error: %w", err)
	}

	return items, nil
}

// addItem inserts a new item into the database
// Uses parameterized queries to prevent SQL injection.
// Uses the global dbpool (DBPool interface)
func addItem(ctx context.Context, newItem Item) (Item, error) {
	// Basic validation (could be more extensive)
	if strings.TrimSpace(newItem.Name) == "" || strings.TrimSpace(newItem.Quantity) == "" {
		return Item{}, fmt.Errorf("item name and quantity cannot be empty")
	}

	var insertedID int
	var createdAt time.Time
	// Use QueryRow method from the DBPool interface
	err := dbpool.QueryRow(ctx,
		"INSERT INTO items (name, quantity) VALUES ($1, $2) RETURNING id, created_at",
		newItem.Name, newItem.Quantity, // Parameters are handled safely by pgx
	).Scan(&insertedID, &createdAt)

	if err != nil {
		log.Printf("Error inserting item: %v\n", err)
		return Item{}, fmt.Errorf("database insert error: %w", err)
	}

	newItem.ID = insertedID
	newItem.CreatedAt = createdAt
	log.Printf("Added item: ID=%d, Name=%s, Quantity=%s\n", newItem.ID, newItem.Name, newItem.Quantity)
	return newItem, nil
}

// deleteItem removes an item from the database by ID
// Uses parameterized queries.
// Uses the global dbpool (DBPool interface)
func deleteItem(ctx context.Context, id int) error {
	// Use Exec method from the DBPool interface
	cmdTag, err := dbpool.Exec(ctx, "DELETE FROM items WHERE id = $1", id)
	if err != nil {
		log.Printf("Error deleting item with ID %d: %v\n", id, err)
		return fmt.Errorf("database delete error: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		log.Printf("Attempted to delete non-existent item with ID %d\n", id)
		// Return a distinct error for not found if needed by caller
		return fmt.Errorf("item with ID %d not found", id)
	}
	log.Printf("Deleted item with ID %d\n", id)
	return nil
}

// --- HTTP Handlers ---
// Handlers remain the same, they internally call the DB functions which now use the interface

func itemsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getItemsHandler(w, r)
	case http.MethodPost:
		addItemHandler(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func itemDetailHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path like /api/items/123
	// Ensure path ends with the ID and not just /items/
	pathParts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[len(pathParts)-1] == "" || pathParts[len(pathParts)-2] != "items" {
		http.Error(w, "Bad Request: Invalid URL format or missing item ID", http.StatusBadRequest)
		return
	}
	idStr := pathParts[len(pathParts)-1]

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request: Invalid item ID format", http.StatusBadRequest)
		return
	}

	// Now handle the method
	switch r.Method {
	case http.MethodDelete:
		deleteItemHandler(w, r, id) // Pass the parsed ID
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func getItemsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := getItems(r.Context())
	if err != nil {
		log.Printf("Error in getItemsHandler: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Handle case where items might be nil if getItems returns nil on error
	if items == nil {
		items = []Item{} // Return empty array instead of null JSON
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Printf("Error encoding items to JSON: %v", err)
		// Avoid writing header again if already written by Encode
	}
}

func addItemHandler(w http.ResponseWriter, r *http.Request) {
	var newItem Item
	// Decode JSON request body
	// Use http.MaxBytesReader to prevent large request bodies (DoS protection)
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Prevent extra fields in JSON

	if err := dec.Decode(&newItem); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError // Check for body too large

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at character %d)", syntaxError.Offset)
			http.Error(w, msg, http.StatusBadRequest)
		case errors.Is(err, io.ErrUnexpectedEOF):
			http.Error(w, "Request body contains badly-formed JSON", http.StatusBadRequest)
		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at character %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			http.Error(w, msg, http.StatusBadRequest)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			http.Error(w, msg, http.StatusBadRequest)
		case errors.Is(err, io.EOF): // Empty body
			http.Error(w, "Request body must not be empty", http.StatusBadRequest)
		case errors.As(err, &maxBytesError):
			http.Error(w, "Request body must not be larger than 1MB", http.StatusRequestEntityTooLarge)
		default: // Catch-all for other decoding errors
			log.Printf("Error decoding JSON body: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError) // Keep internal errors internal
		}
		return
	}

	// Input validation is handled within addItem
	addedItem, err := addItem(r.Context(), newItem)
	if err != nil {
		log.Printf("Error adding item: %v", err)
		if strings.Contains(err.Error(), "cannot be empty") {
			http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		} else {
			// Other DB errors are internal
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created
	if err := json.NewEncoder(w).Encode(addedItem); err != nil {
		log.Printf("Error encoding added item to JSON: %v", err)
	}
}

// deleteItemHandler now receives the parsed ID
func deleteItemHandler(w http.ResponseWriter, r *http.Request, id int) {
	err := deleteItem(r.Context(), id)
	if err != nil {
		log.Printf("Error deleting item %d: %v", id, err)
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content is typical for successful DELETE
}

// --- Main Function ---

func main() {
	// Optional: Load .env file for local development
	if os.Getenv("APP_ENV") != "production" {
		err := godotenv.Load()
		if err != nil {
			log.Println("No .env file found or error loading, proceeding with environment variables")
		}
	}

	// Database Configuration from Environment Variables
	dbPort, _ := strconv.Atoi(getenv("DB_PORT", "5432"))
	dbConfig := DBConfig{
		Host:     getenv("DB_HOST", "db"),
		Port:     dbPort,
		User:     getenv("DB_USER", "user"),
		Password: getenv("DB_PASSWORD", "password"),
		DBName:   getenv("DB_NAME", "shoppingdb"),
		SSLMode:  getenv("DB_SSLMODE", "disable"),
	}

	// Connect to Database and setup pooling
	// pool is the concrete *pgxpool.Pool type
	pool, err := connectDB(dbConfig)
	if err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}
	// Assign the concrete pool to the global DBPool interface variable.
	// This works because *pgxpool.Pool implements the DBPool interface.
	dbpool = pool
	// VERY IMPORTANT: Defer Close() on the CONCRETE pool object returned by connectDB.
	// If you defer dbpool.Close(), it might work, but it's less explicit.
	// Closing the concrete pool handles the actual resource cleanup.
	defer pool.Close()

	// Create Schema if it doesn't exist, using the interface variable
	if err := createSchemaIfNotExists(dbpool); err != nil {
		log.Fatalf("Could not create database schema: %v", err)
	}

	// Setup HTTP Router
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/items", itemsHandler)       // Handles GET /items, POST /items
	mux.HandleFunc("/items/", itemDetailHandler) // Handles DELETE /items/{id}

	// Health Check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Use the global dbpool (interface) for pinging
		if err := dbpool.Ping(r.Context()); err != nil {
			log.Printf("Health check failed: %v", err) // Log the specific error
			http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// Start HTTP Server
	port := getenv("APP_PORT", "8080")
	serverAddr := ":" + port
	log.Printf("Starting server on %s\n", serverAddr)

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Could not listen on %s: %v\n", serverAddr, err)
	}
}

// Helper function to get environment variables with a default value
func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Printf("Environment variable %s not set, using default: %s", key, fallback)
	return fallback
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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

// --- Global Variables ---
var dbpool *pgxpool.Pool

// --- Database Functions ---

// connectDB initializes the database connection pool
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
func createSchemaIfNotExists(pool *pgxpool.Pool) error {
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
func getItems(ctx context.Context) ([]Item, error) {
	rows, err := dbpool.Query(ctx, "SELECT id, name, quantity, created_at FROM items ORDER BY created_at DESC")
	if err != nil {
		// If no rows are found, return an empty slice, not an error
		if err == sql.ErrNoRows { // Note: pgx might return pgx.ErrNoRows, check specific error if needed
			return []Item{}, nil
		}
		log.Printf("Error querying items: %v\n", err)
		return nil, fmt.Errorf("database query error: %w", err)
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Quantity, &item.CreatedAt); err != nil {
			log.Printf("Error scanning item row: %v\n", err)
			// Decide whether to skip this row or return an error
			// For robustness, potentially skip and log
			continue // Skip this item if scanning fails
		}
		items = append(items, item)
	}

	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Printf("Error after iterating rows: %v\n", err)
		return nil, fmt.Errorf("database iteration error: %w", err)
	}

	return items, nil
}

// addItem inserts a new item into the database
// Uses parameterized queries to prevent SQL injection.
func addItem(ctx context.Context, newItem Item) (Item, error) {
	// Basic validation (could be more extensive)
	if strings.TrimSpace(newItem.Name) == "" || strings.TrimSpace(newItem.Quantity) == "" {
		return Item{}, fmt.Errorf("item name and quantity cannot be empty")
	}

	var insertedID int
	var createdAt time.Time
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
func deleteItem(ctx context.Context, id int) error {
	cmdTag, err := dbpool.Exec(ctx, "DELETE FROM items WHERE id = $1", id)
	if err != nil {
		log.Printf("Error deleting item with ID %d: %v\n", id, err)
		return fmt.Errorf("database delete error: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		log.Printf("Attempted to delete non-existent item with ID %d\n", id)
		// Consider returning a specific error like ErrNotFound if needed by the frontend
		return fmt.Errorf("item with ID %d not found", id) // Or return nil if "not found" isn't an error case here
	}
	log.Printf("Deleted item with ID %d\n", id)
	return nil
}

// --- HTTP Handlers ---

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
	switch r.Method {
	case http.MethodDelete:
		deleteItemHandler(w, r)
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
		// Don't write header again if already written by Encode
		// http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
		log.Printf("Error decoding JSON body: %v", err)
		http.Error(w, "Bad Request: Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Input is implicitly sanitized against XSS *here* because we are treating
	// Name and Quantity as plain text data for the DB.
	// SQL injection is prevented by using parameterized queries in `addItem`.
	// XSS prevention is primarily needed when *rendering* this data back into HTML.
	// The frontend's escapeHtml function handles display-side XSS.

	addedItem, err := addItem(r.Context(), newItem)
	if err != nil {
		log.Printf("Error adding item: %v", err)
		// Check for specific errors if needed (e.g., validation error vs db error)
		if strings.Contains(err.Error(), "cannot be empty") {
			http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		} else {
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

func deleteItemHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path like /api/items/123
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 || pathParts[len(pathParts)-1] == "" {
		http.Error(w, "Bad Request: Missing item ID in URL", http.StatusBadRequest)
		return
	}
	idStr := pathParts[len(pathParts)-1]

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request: Invalid item ID", http.StatusBadRequest)
		return
	}

	err = deleteItem(r.Context(), id)
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
	// In production (Docker), rely on environment variables set in docker-compose.yml
	if os.Getenv("APP_ENV") != "production" {
		err := godotenv.Load() // Load .env file from current directory
		if err != nil {
			log.Println("No .env file found, proceeding with environment variables")
		}
	}

	// Database Configuration from Environment Variables
	dbPort, _ := strconv.Atoi(getenv("DB_PORT", "5432"))
	dbConfig := DBConfig{
		Host:     getenv("DB_HOST", "db"), // 'db' is the service name in docker-compose
		Port:     dbPort,
		User:     getenv("DB_USER", "user"),
		Password: getenv("DB_PASSWORD", "password"),
		DBName:   getenv("DB_NAME", "shoppingdb"),
		SSLMode:  getenv("DB_SSLMODE", "disable"), // Use "require" or others if needed
	}

	// Connect to Database and setup pooling
	var err error
	dbpool, err = connectDB(dbConfig)
	if err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}
	defer dbpool.Close() // Ensure pool is closed on application exit

	// Create Schema if it doesn't exist
	if err := createSchemaIfNotExists(dbpool); err != nil {
		log.Fatalf("Could not create database schema: %v", err)
	}

	// Setup HTTP Router
	mux := http.NewServeMux()

	// API Routes
	// Note: Nginx will handle CORS headers based on nginx.conf
	mux.HandleFunc("/items", itemsHandler)       // Handles GET /items, POST /items
	mux.HandleFunc("/items/", itemDetailHandler) // Handles DELETE /items/{id}

	// Health Check endpoint (optional but good practice)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := dbpool.Ping(r.Context()); err != nil {
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
		Handler:      mux, // Use our mux
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

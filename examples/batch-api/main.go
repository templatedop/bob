package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	bobpgx "github.com/templatedop/bob/drivers/pgx"
)

// Product represents a product in the system
type Product struct {
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Stock      int     `json:"stock"`
	CategoryID int     `json:"category_id"`
}

// InsertedProduct represents a product after insertion
type InsertedProduct struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Price      float64   `json:"price"`
	Stock      int       `json:"stock"`
	CategoryID int       `json:"category_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// API holds the application state
type API struct {
	pool bobpgx.Pool
}

// NewAPI creates a new API instance
func NewAPI(pool bobpgx.Pool) *API {
	return &API{pool: pool}
}

// BulkInsertProducts inserts multiple products using batch operations
func (api *API) BulkInsertProducts(ctx context.Context, products []Product) ([]InsertedProduct, error) {
	// Acquire connection from pool
	conn, err := api.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	// Create batch - NO explicit transaction needed!
	// pgx batches are implicitly transactional
	batch := &pgx.Batch{}

	// Queue all insert statements
	for _, p := range products {
		batch.Queue(
			"INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4) RETURNING id, created_at",
			p.Name, p.Price, p.Stock, p.CategoryID,
		)
	}

	// Send batch - all operations execute atomically
	results := conn.SendBatch(ctx, batch)
	defer results.Close()

	// Collect results with RETURNING data
	inserted := make([]InsertedProduct, 0, len(products))
	for i, p := range products {
		var id int
		var createdAt time.Time

		err := results.QueryRow().Scan(&id, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("insert product %d (%s): %w", i, p.Name, err)
		}

		inserted = append(inserted, InsertedProduct{
			ID:         id,
			Name:       p.Name,
			Price:      p.Price,
			Stock:      p.Stock,
			CategoryID: p.CategoryID,
			CreatedAt:  createdAt,
		})
	}

	return inserted, nil
}

// Request/Response types
type BulkProductRequest struct {
	Products []Product `json:"products"`
}

type BulkProductResponse struct {
	Inserted   int               `json:"inserted"`
	DurationMs int64             `json:"duration_ms"`
	Message    string            `json:"message"`
	Products   []InsertedProduct `json:"products"`
}

type ProductListResponse struct {
	Products []InsertedProduct `json:"products"`
	Count    int               `json:"count"`
}

// HTTP Handlers

// HandleBulkInsert handles POST /api/products/bulk
func (api *API) HandleBulkInsert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req BulkProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.Products) == 0 {
		http.Error(w, "No products provided", http.StatusBadRequest)
		return
	}

	// Validate products
	for i, p := range req.Products {
		if p.Name == "" {
			http.Error(w, fmt.Sprintf("Product %d: name is required", i), http.StatusBadRequest)
			return
		}
		if p.Price <= 0 {
			http.Error(w, fmt.Sprintf("Product %d: price must be positive", i), http.StatusBadRequest)
			return
		}
		if p.Stock < 0 {
			http.Error(w, fmt.Sprintf("Product %d: stock cannot be negative", i), http.StatusBadRequest)
			return
		}
	}

	start := time.Now()

	// Insert using batch operations
	inserted, err := api.BulkInsertProducts(ctx, req.Products)
	if err != nil {
		log.Printf("BulkInsertProducts error: %v", err)
		http.Error(w, "Failed to insert products", http.StatusInternalServerError)
		return
	}

	duration := time.Since(start)

	// Return success response
	resp := BulkProductResponse{
		Inserted:   len(inserted),
		DurationMs: duration.Milliseconds(),
		Message:    fmt.Sprintf("Successfully inserted %d products", len(inserted)),
		Products:   inserted,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)

	log.Printf("Bulk inserted %d products in %dms", len(inserted), duration.Milliseconds())
}

// HandleListProducts handles GET /api/products
func (api *API) HandleListProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := api.pool.QueryContext(ctx,
		"SELECT id, name, price, stock, category_id, created_at FROM products ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		log.Printf("Query error: %v", err)
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := make([]InsertedProduct, 0)
	for rows.Next() {
		var p InsertedProduct
		err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.CategoryID, &p.CreatedAt)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		products = append(products, p)
	}

	resp := ProductListResponse{
		Products: products,
		Count:    len(products),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleGetProduct handles GET /api/products/:id
func (api *API) HandleGetProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	rows, err := api.pool.QueryContext(ctx,
		"SELECT id, name, price, stock, category_id, created_at FROM products WHERE id = $1", id)
	if err != nil {
		log.Printf("Query error: %v", err)
		http.Error(w, "Failed to fetch product", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	if !rows.Next() {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	var p InsertedProduct
	err = rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.CategoryID, &p.CreatedAt)
	if err != nil {
		log.Printf("Scan error: %v", err)
		http.Error(w, "Failed to scan product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// HandlePerformanceTest handles POST /api/products/bulk/test
func (api *API) HandlePerformanceTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	countStr := r.URL.Query().Get("count")
	count := 100
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 || count > 10000 {
			http.Error(w, "Invalid count (must be 1-10000)", http.StatusBadRequest)
			return
		}
	}

	// Generate test products
	products := make([]Product, count)
	for i := 0; i < count; i++ {
		products[i] = Product{
			Name:       fmt.Sprintf("Test Product %d", i+1),
			Price:      float64(10 + (i % 90)),
			Stock:      100 + (i % 200),
			CategoryID: 1 + (i % 4),
		}
	}

	start := time.Now()
	inserted, err := api.BulkInsertProducts(ctx, products)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Performance test error: %v", err)
		http.Error(w, "Failed to insert test products", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"count":        len(inserted),
		"duration_ms":  duration.Milliseconds(),
		"ops_per_sec":  float64(len(inserted)) / duration.Seconds(),
		"message":      fmt.Sprintf("Inserted %d products in %dms (%.0f ops/sec)", len(inserted), duration.Milliseconds(), float64(len(inserted))/duration.Seconds()),
		"sample_products": inserted[:min(5, len(inserted))],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	log.Printf("Performance test: inserted %d products in %dms (%.0f ops/sec)",
		len(inserted), duration.Milliseconds(), float64(len(inserted))/duration.Seconds())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	ctx := context.Background()

	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://batchuser:batchpass@localhost:5432/batchapi?sslmode=disable"
	}

	// Create connection pool
	pool, err := bobpgx.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✓ Connected to database")

	// Create API
	api := NewAPI(pool)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// Routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Batch Operations API",
			"version": "1.0.0",
			"endpoints": "/api/products, /api/products/bulk, /api/products/bulk/test",
		})
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/products", api.HandleListProducts)
		r.Get("/products/{id}", api.HandleGetProduct)
		r.Post("/products/bulk", api.HandleBulkInsert)
		r.Post("/products/bulk/test", api.HandlePerformanceTest)
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("✓ Server starting on http://localhost%s", addr)
	log.Printf("  Endpoints:")
	log.Printf("    GET  http://localhost%s/api/products", addr)
	log.Printf("    GET  http://localhost%s/api/products/:id", addr)
	log.Printf("    POST http://localhost%s/api/products/bulk", addr)
	log.Printf("    POST http://localhost%s/api/products/bulk/test?count=100", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Version is set via build arg (-ldflags).
var Version = "v1"

var db *sql.DB

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Connect to Postgres if DATABASE_URL is set
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		var err error
		db, err = sql.Open("postgres", dbURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
			os.Exit(1)
		}
		if err := db.Ping(); err != nil {
			fmt.Printf("Warning: database not ready yet: %v (will retry on requests)\n", err)
		} else {
			fmt.Println("Connected to database")
		}
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	http.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"version": Version})
	})

	http.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			msg = "hello"
		}
		writeJSON(w, map[string]string{"echo": msg, "version": Version})
	})

	// Items CRUD (requires database)
	http.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, `{"error":"database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleGetItems(w, r)
		case http.MethodPost:
			handleCreateItem(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})

	// Start HTTP Server
	go func() {
		log.Printf("Starting HTTP server on :%s\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on :50051: %v", err)
	}
	s := grpc.NewServer()

	// Register Health Service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("topas.EchoService", grpc_health_v1.HealthCheckResponse_SERVING) // Logic placeholder

	// Register Reflection Service (Critical for dynamic invocation)
	reflection.Register(s)

	log.Printf("Starting gRPC server on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

type Item struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func handleGetItems(w http.ResponseWriter, r *http.Request) {
	nameFilter := r.URL.Query().Get("name")

	var rows *sql.Rows
	var err error
	if nameFilter != "" {
		rows, err = db.Query("SELECT id, name, price FROM items WHERE name = $1", nameFilter)
	} else {
		rows, err = db.Query("SELECT id, name, price FROM items")
	}
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Price); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}

	writeJSON(w, map[string]interface{}{"items": items, "count": len(items)})
}

func handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
		return
	}

	var id int
	err := db.QueryRow("INSERT INTO items (name, price) VALUES ($1, $2) RETURNING id", input.Name, input.Price).Scan(&id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, Item{ID: id, Name: input.Name, Price: input.Price})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

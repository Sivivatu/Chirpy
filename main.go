package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"sivivatu/Chirpy/chirpy/internal/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQuery        *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
		cfg.fileserverHits.Store(0)
		err := cfg.dbQuery.ResetDatabase(r.Context())
		if err != nil {
			log.Printf("Error resetting database: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
	w.Write([]byte("Hits reset to 0"))
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}

	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func censorBody(body string) string {
	words := strings.Split(string(body), " ")

	banned := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	for i := 0; i < len(words); i++ {
		loweredWord := strings.ToLower(words[i])
		if _, ok := banned[loweredWord]; ok {
			words[i] = "****"
		}
	}

	return strings.Join(words, " ")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: could not load .env file: %s", err)
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL environment variable is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error loading database: %s", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Error connecting to database: %s", err)
	}
	dbQueries := database.New(db)

	cfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		dbQuery:        dbQueries,
	}
	mux := http.NewServeMux()

	// General web endpoints
	mux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))
	mux.Handle("/assets/", cfg.middlewareMetricsInc(http.StripPrefix("/assets/", http.FileServer(http.Dir(".")))))

	// Admin Endpoints
	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write(
			[]byte(fmt.Sprintf(`
				<html>
					<body>
						<h1>Welcome, Chirpy Admin</h1>
						<p>Chirpy has been visited %d times!</p>
					</body>
				</html>
			`, cfg.fileserverHits.Load())),
		)
	})
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)

	// Api endpoints
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Body string `json:"body"`
		}

		type validResponse struct {
			Cleaned_Body string `json:"cleaned_body"`
		}

		decoder := json.NewDecoder(r.Body)
		chirp := requestBody{}
		err := decoder.Decode(&chirp)

		if err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		if len(chirp.Body) > 140 {
			respondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}

		cleaned := censorBody(chirp.Body)

		respondWithJSON(w, http.StatusOK, validResponse{
			Cleaned_Body: cleaned,
		})
	})

	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Email string `json:"email"`
		}

		decoder := json.NewDecoder(r.Body)
		user := requestBody{}
		err := decoder.Decode(&user)
		
		if err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
		
		createdUser, err := cfg.dbQuery.CreateUser(r.Context(), user.Email)
		if err != nil {
			log.Printf("Error creating user: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		respondWithJSON(w, http.StatusCreated, map[string]string{
			"id":         createdUser.ID.String(),
			"created_at": createdUser.CreatedAt.Format(time.RFC3339),
			"updated_at": createdUser.UpdatedAt.Format(time.RFC3339),
			"email":      createdUser.Email,
		})
		
	})

	// create a new chirp
	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			Body string `json:"body"`
			UserID string `json:"user_id"`
		}

		decoder := json.NewDecoder(r.Body)
		chirp := requestBody{}
		err := decoder.Decode(&chirp)
		
		if err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}
		
		userID, err := uuid.Parse(chirp.UserID)
		if err != nil {
			log.Printf("Error parsing user ID: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid user ID")
			return
		}

		createdChirp, err := cfg.dbQuery.CreateChirp(r.Context(), chirp.Body, userID)
		if err != nil {
			log.Printf("Error creating chirp: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Something went wrong")
			return
		}

		respondWithJSON(w, http.StatusCreated, map[string]string{
			"id":         createdChirp.ID.String(),
			"created_at": createdChirp.CreatedAt.Format(time.RFC3339),
			"updated_at": createdChirp.UpdatedAt.Format(time.RFC3339),
			"body":       createdChirp.Body,
			"user_id":    createdChirp.UserID.String(),
		})
	})

	// get all chirps
	mux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		chirps, err := cfg.dbQuery.GetChirps(r.Context())
		if err != nil {
			log.Printf("Error getting chirps: %s", err)
			respondWithError(w, http.StatusNotFound, "Chirps not found")
			return
		}

		type chirpResponse struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
			Body      string `json:"body"`
			UserID    string `json:"user_id"`
		}
		
		resp := make([]chirpResponse, 0, len(chirps))
		for _, chirp := range chirps {
			resp = append(resp, chirpResponse{
				ID:        chirp.ID.String(),
				CreatedAt: chirp.CreatedAt.Format(time.RFC3339),
				UpdatedAt: chirp.UpdatedAt.Format(time.RFC3339),
				Body:      chirp.Body,
				UserID:    chirp.UserID.String(),
			})
		}
		respondWithJSON(w, http.StatusOK, resp)
	})

	// get a single chirp
	mux.HandleFunc("GET /api/chirps/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		chirpID, err := uuid.Parse(id)
		if err != nil {
			log.Printf("Error parsing chirp ID: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid chirp ID")
			return
		}
		chirp, err := cfg.dbQuery.GetChirp(r.Context(), chirpID)
		if err != nil {
			log.Printf("Error getting chirp: %s", err)
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]string{
			"id":         chirp.ID.String(),
			"created_at": chirp.CreatedAt.Format(time.RFC3339),
			"updated_at": chirp.UpdatedAt.Format(time.RFC3339),
			"body":       chirp.Body,
			"user_id":    chirp.UserID.String(),
		})
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	server.ListenAndServe()
}

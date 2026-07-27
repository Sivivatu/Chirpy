package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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


func main() {
	cfg := &apiConfig{
		fileserverHits: atomic.Int32{},
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

	// Admin endpoints
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
			Valid bool `json:"valid"`
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

		respondWithJSON(w, http.StatusOK, validResponse{
			Valid: true,
		})
	})

	server := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	// Website Serving 
	server.ListenAndServe()
}
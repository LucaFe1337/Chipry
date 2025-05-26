package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/LucaFe1337/Chipry/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	DB             *database.Queries
	PLATFORM       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	accessNum := cfg.fileserverHits.Load()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	htmlResponse := fmt.Sprintf(`
	<html>
  		<body>
    		<h1>Welcome, Chirpy Admin</h1>
    		<p>Chirpy has been visited %d times!</p>
  		</body>
	</html>`, accessNum)
	fmt.Fprint(w, htmlResponse)
}

func (cfg *apiConfig) resetMetrics(w http.ResponseWriter, r *http.Request) {
	if cfg.PLATFORM == "dev" {
		cfg.fileserverHits.Store(0)
		err := cfg.DB.DeleteAllUsers(r.Context())
		if err != nil {
			fmt.Fprint(w, "Error deleting users!")
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Count reset!")
	} else {
		w.WriteHeader(http.StatusForbidden)
		return
	}

}

func respondWithError(w http.ResponseWriter, code int, message string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	resp := errorResponse{Error: message}
	dat, err := json.Marshal(resp)
	if err != nil {
		fmt.Printf("Error marshaling JSON error response: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling JSON response: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func validateChirps(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	param := parameters{}
	err := decoder.Decode(&param)
	if err != nil {
		// Fehler beim Dekodieren des JSON-Bodies
		// Der Client hat ungültiges JSON gesendet
		respondWithError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	chirpText := param.Body
	maxLength := 140

	if len(chirpText) > maxLength {
		// Chirp ist zu lang
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Chirp is too long. Max length is %d characters.", maxLength))
		return
	}
	cleanedChirp := cleanChirp(chirpText)

	type successResponse struct {
		Cleaned_Body string `json:"cleaned_body"`
	}

	resp := successResponse{
		Cleaned_Body: cleanedChirp,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func cleanChirp(text string) string {
	badWords := map[string]string{
		"kerfuffle": "****",
		"sharbert":  "****",
		"fornax":    "****",
		"Kerfuffle": "****",
		"Sharbert":  "****",
		"Fornax":    "****",
	}

	cleanedText := text
	for bad, replacement := range badWords {
		cleanedText = strings.ReplaceAll(cleanedText, bad, replacement)
	}
	return cleanedText
}

func (cfg *apiConfig) createNewUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	param := parameters{}
	err := decoder.Decode(&param)
	if err != nil {
		// Fehler beim Dekodieren des JSON-Bodies
		// Der Client hat ungültiges JSON gesendet
		respondWithError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	user, err := cfg.DB.CreateUser(r.Context(), param.Email)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "smth went wrong Creating the user!")
		return
	}
	resp := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	respondWithJSON(w, http.StatusCreated, resp)
}

func main() {
	godotenv.Load()
	mux := http.NewServeMux()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error in DB!", err)
	}
	dbQueries := database.New(db)
	platform := os.Getenv("PLATFORM")

	fs := http.FileServer(http.Dir("."))

	apiCfg := apiConfig{
		DB:       dbQueries,
		PLATFORM: platform,
	}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", fs)))
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetMetrics)
	mux.HandleFunc("POST /api/validate_chirp", validateChirps)
	mux.HandleFunc("POST /api/users", apiCfg.createNewUser)

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	fmt.Println("Server is listening on localhost:8080")
	err = server.ListenAndServe()
	if err != nil {
		fmt.Println("Server error", err)
	}

}

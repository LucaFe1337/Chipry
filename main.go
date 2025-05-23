package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Count reset!")
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

func main() {
	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir("."))

	apiCfg := apiConfig{}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", fs)))
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetMetrics)
	mux.HandleFunc("POST /api/validate_chirp", validateChirps)

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
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Server error", err)
	}

}

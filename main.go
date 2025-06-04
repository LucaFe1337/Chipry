package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/LucaFe1337/Chipry/internal/auth"
	"github.com/LucaFe1337/Chipry/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	DB             *database.Queries
	PLATFORM       string
	Secret         string
	POLKA_API_KEY  string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Red       bool      `json:"is_chirpy_red"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	User_id   uuid.UUID `json:"user_id"`
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

func validateChirps(chirpText string) (string, error) {
	maxLength := 140

	if len(chirpText) > maxLength {
		// Chirp ist zu lang
		return "", fmt.Errorf("Chirp is too long. Max length is %d characters.", maxLength)
	}
	cleanedChirp := cleanChirp(chirpText)

	type successResponse struct {
		Cleaned_Body string `json:"cleaned_body"`
	}

	return cleanedChirp, nil
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
		Email    string `json:"email"`
		Password string `json:"password"`
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

	var userParams database.CreateUserParams
	userParams.Email = param.Email
	userParams.HashedPassword, err = auth.HashPassword(param.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Erorr trying to Hash the password!")
		return
	}

	user, err := cfg.DB.CreateUser(r.Context(), userParams)
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

func (cfg *apiConfig) postChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	param := parameters{}
	err := decoder.Decode(&param)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}
	token_string, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Token invalid")
		return
	}
	userID, err := auth.ValidateJWT(token_string, cfg.Secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Token invalid")
		return
	}

	chirpText, err := validateChirps(param.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Valdation of Chirp failed!")
		return
	}
	var chirpdata database.CreateChirpsParams
	chirpdata.Body = chirpText
	chirpdata.UserID = userID
	chirp, err := cfg.DB.CreateChirps(r.Context(), chirpdata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "smth went wrong Creating the Chirp!")
		return
	}
	resp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		User_id:   chirp.UserID,
	}
	respondWithJSON(w, http.StatusCreated, resp)
}

func (cfg *apiConfig) GetAllChirps(w http.ResponseWriter, r *http.Request) {
	author_id := r.URL.Query().Get("author_id")
	sorting := r.URL.Query().Get("sort")
	var chirps []database.Chirp
	if author_id == "" {
		var innererror error
		chirps, innererror = cfg.DB.Allchirps(r.Context())
		if innererror != nil {
			respondWithError(w, http.StatusInternalServerError, "Error retrieving chirps")
			return
		}
	} else {
		parsed_user_id, err := uuid.Parse(author_id)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error parsing author id to uuid")
			return
		}
		chirps, err = cfg.DB.AllchirpsFromUser(r.Context(), parsed_user_id)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error retrieving chirps from specified user")
			return
		}
	}

	var allChirps []Chirp
	for _, chirp := range chirps {
		newChirp := Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			User_id:   chirp.UserID,
		}
		allChirps = append(allChirps, newChirp)
	}
	if sorting == "desc" {
		sort.Slice(allChirps, func(i, j int) bool { return allChirps[i].CreatedAt.After(allChirps[j].CreatedAt) })
	}
	respondWithJSON(w, http.StatusOK, allChirps)
}
func (cfg *apiConfig) getChipById(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	id, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing ID!")
		return
	}

	chirp, err := cfg.DB.GetChirpById(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Error retrieving chirp")
		return
	}
	newChirp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		User_id:   chirp.UserID,
	}
	respondWithJSON(w, http.StatusOK, newChirp)
}

func (cfg *apiConfig) authenticateLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
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
	user, err := cfg.DB.GetPasswordFromEmail(r.Context(), param.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error retrieving user data")
		return
	}
	err = auth.CheckPassword(user.HashedPassword, param.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Wrong Password!")
		return
	}
	token_string, err := auth.MakeJWT(user.ID, cfg.Secret, time.Duration(3600)*time.Second)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating token string")
		return
	}
	token, _ := auth.MakeRefreshToken()
	refresh_token_params := database.CreateRefreshTokenParams{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().AddDate(0, 0, 60),
	}
	refresh_token, err := cfg.DB.CreateRefreshToken(r.Context(), refresh_token_params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating refresh token")
		return
	}
	resp := struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		Red          bool      `json:"is_chirpy_red"`
	}{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token_string,
		RefreshToken: refresh_token.Token,
		Red:          user.IsChirpyRed,
	}
	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) refreshToken(w http.ResponseWriter, r *http.Request) {
	refresh_token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		fmt.Printf("refresh token wrong format: %s", refresh_token)
		respondWithError(w, http.StatusUnauthorized, "Error receiving refesh token:")
		return
	}
	data, err := cfg.DB.GetUserFromRefreshToken(r.Context(), refresh_token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "error receiving user and refresh token data")
		return
	}
	if data.RevokedAt.Valid {
		respondWithError(w, http.StatusUnauthorized, "refresh token revoked")
		return
	}
	if data.ExpiresAt.Before(time.Now()) {
		respondWithError(w, http.StatusUnauthorized, "refresh token expired")
		return
	}
	access_token, err := auth.MakeJWT(data.ID, cfg.Secret, time.Duration(3600)*time.Second)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating acces token")
		return
	}

	resp := struct {
		Token string `json:"token"`
	}{
		Token: access_token,
	}
	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) revokeRefreshToken(w http.ResponseWriter, r *http.Request) {
	refresh_token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error receiving refesh token, in revoke refresh")
		return
	}
	var revoke_params database.RevokeRefreshTokenParams
	revoke_params.RevokedAt.Time = time.Now()
	revoke_params.RevokedAt.Valid = true
	revoke_params.Token = refresh_token

	err = cfg.DB.RevokeRefreshToken(r.Context(), revoke_params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error revoking token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) changeUserData(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	access_token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error receiving refesh token, in revoke refresh")
		return
	}
	decoder := json.NewDecoder(r.Body)
	param := parameters{}
	err = decoder.Decode(&param)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}
	var newUserData database.UpdateUserDataParams
	userID, err := auth.ExtractUserIDFromJWT(access_token, cfg.Secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "error parsing userID from token")
		return
	}
	newUserData.ID = userID
	newUserData.Email = param.Email
	new_hashed_passwd, err := auth.HashPassword(param.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error hashing new password")
	}
	newUserData.HashedPassword = new_hashed_passwd
	newUserData.UpdatedAt = time.Now()
	err = cfg.DB.UpdateUserData(r.Context(), newUserData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating user data")
		return
	}
	resp := User{
		ID:        userID,
		CreatedAt: newUserData.UpdatedAt,
		UpdatedAt: newUserData.UpdatedAt,
		Email:     param.Email,
	}

	respondWithJSON(w, http.StatusOK, resp)
}

func (cfg *apiConfig) deleteChirpyById(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	parsed_chirpID, err := uuid.Parse(chirpID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error parsing chirpID into uuid")
		return
	}
	// get chirp
	chirp, err := cfg.DB.GetChirpById(r.Context(), parsed_chirpID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "error getting chirp out of db")
		return
	}
	// get user data
	access_token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error receiving refresh token, in revoke refresh")
		return
	}
	userID, err := auth.ExtractUserIDFromJWT(access_token, cfg.Secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "error parsing userID from token")
		return
	}
	if userID != chirp.UserID {
		respondWithError(w, http.StatusForbidden, "user is not the author of the chirp, cant delete other users chirps")
		return
	}
	err = cfg.DB.DeleteChripyById(r.Context(), parsed_chirpID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "error deleting chirp, not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type data struct {
	UserId uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) UpgradeUserToRed(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Event string `json:"event"`
		Data  data   `json:"data"`
	}
	api_key, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "api key not found in header")
		return
	}
	if api_key != cfg.POLKA_API_KEY {
		respondWithError(w, http.StatusUnauthorized, "api key doesnt match")
		return
	}
	decoder := json.NewDecoder(r.Body)
	param := parameters{}
	err = decoder.Decode(&param)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}
	if param.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	err = cfg.DB.UpdateUserToRed(r.Context(), param.Data.UserId)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "user not found, couldnt upgrade")
	}
	w.WriteHeader(http.StatusNoContent)
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
	secret := os.Getenv("SECRET")
	polka_api_key := os.Getenv("POLKA_KEY")

	fs := http.FileServer(http.Dir("."))

	apiCfg := apiConfig{
		DB:            dbQueries,
		PLATFORM:      platform,
		Secret:        secret,
		POLKA_API_KEY: polka_api_key,
	}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app/", fs)))
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetMetrics)
	mux.HandleFunc("POST /api/users", apiCfg.createNewUser)
	mux.HandleFunc("POST /api/chirps", apiCfg.postChirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.GetAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChipById)
	mux.HandleFunc("POST /api/login", apiCfg.authenticateLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.refreshToken)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeRefreshToken)
	mux.HandleFunc("PUT /api/users", apiCfg.changeUserData)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirpyById)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.UpgradeUserToRed)

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

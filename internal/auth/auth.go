// All Authentication Code here
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashed_password, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return "", err
	}
	return string(hashed_password), nil
}

func CheckPassword(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	var claims jwt.RegisteredClaims
	claims.Issuer = "chirpy"
	claims.IssuedAt = jwt.NewNumericDate(time.Now().UTC())
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().UTC().Add(expiresIn))
	claims.Subject = userID.String()

	new_token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token_string, err := new_token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return token_string, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims

	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("an Error has ocurred: %v", err)
	}
	if !token.Valid {
		return uuid.Nil, fmt.Errorf("token was not valid")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("an Error has ocurred: %v", err)
	}
	return userID, nil
}

func ExtractUserIDFromJWT(tokenString string, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Make sure the signing method is what you expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	} else {
		return uuid.Nil, errors.New("invalid token claims")
	}
}

func GetBearerToken(headers http.Header) (string, error) {
	authorization := headers.Get("Authorization")
	if authorization == "" {
		return "", fmt.Errorf("no authorization in the header")
	}
	if !strings.HasPrefix(authorization, "Bearer") {
		return "", fmt.Errorf("bearer in wrong format")
	}
	if len(authorization) < 7 {
		return "", fmt.Errorf("authorization string too short, must be false")
	}
	return authorization[7:], nil
}

func MakeRefreshToken() (string, error) {
	refresh_token_in_byte := make([]byte, 32, 32)
	rand.Read(refresh_token_in_byte)
	refresh_token := hex.EncodeToString(refresh_token_in_byte)
	return refresh_token, nil
}

func GetAPIKey(headers http.Header) (string, error) {
	authorization := headers.Get("Authorization")
	if authorization == "" {
		return "", fmt.Errorf("no authorization in the header")
	}
	if !strings.HasPrefix(authorization, "ApiKey") {
		return "", fmt.Errorf("API Key in wrong format")
	}
	if len(authorization) < 7 {
		return "", fmt.Errorf("authorization string too short, must be false")
	}
	return authorization[7:], nil
}

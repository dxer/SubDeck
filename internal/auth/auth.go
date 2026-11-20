package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecretMu sync.RWMutex
	jwtSecret   []byte
)

// SetJWTSecret configures the signing secret key used for JWT tokens.
func SetJWTSecret(secret string) {
	if len(secret) > 0 {
		jwtSecretMu.Lock()
		jwtSecret = []byte(secret)
		jwtSecretMu.Unlock()
	}
}

func getJWTSecret() []byte {
	jwtSecretMu.RLock()
	s := jwtSecret
	jwtSecretMu.RUnlock()
	if len(s) > 0 {
		return s
	}

	// Fallback to a process-unique random secret if SetJWTSecret was not called (e.g. unit tests)
	jwtSecretMu.Lock()
	defer jwtSecretMu.Unlock()
	if len(jwtSecret) == 0 {
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		jwtSecret = []byte(hex.EncodeToString(buf))
	}
	return jwtSecret
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateToken(username string) (string, error) {
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)), // 7 days
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "subdeck",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return getJWTSecret(), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

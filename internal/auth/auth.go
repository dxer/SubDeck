package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecretMu         sync.RWMutex
	jwtSecret           []byte
	currentTokenVersion atomic.Int64
)

func init() {
	currentTokenVersion.Store(1)
}

// SetTokenVersion sets the minimum active JWT token version (bumping invalidates older tokens).
func SetTokenVersion(v int) {
	if v < 1 {
		v = 1
	}
	currentTokenVersion.Store(int64(v))
}

// GetTokenVersion returns the currently active JWT token version.
func GetTokenVersion() int {
	v := int(currentTokenVersion.Load())
	if v < 1 {
		return 1
	}
	return v
}

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
	Username     string `json:"username"`
	TokenVersion int    `json:"tv,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(username string) (string, error) {
	claims := Claims{
		Username:     username,
		TokenVersion: GetTokenVersion(),
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
		if claims.TokenVersion != GetTokenVersion() {
			return nil, errors.New("token has been revoked")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

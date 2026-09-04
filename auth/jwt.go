package auth

import (
	"crypto/rand"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	tokenDuration = 24 * time.Hour
)

// GetJWTSecret returns the JWT secret, loading from environment variable
// AWARENOW_JWT_SECRET or generating a random one at startup if not configured.
func GetJWTSecret() []byte {
	secret := os.Getenv("AWARENOW_JWT_SECRET")
	if secret != "" {
		return []byte(secret)
	}
	// Generate a random secret at startup if not configured
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		panic("failed to generate random JWT secret: " + err.Error())
	}
	return key
}

// jwtSecret is lazily initialized on first use
var jwtSecret []byte
var secretOnce sync.Once

func secretOnceInit() {
	secretOnce.Do(func() {
		jwtSecret = GetJWTSecret()
	})
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateJWT(userID uint, username, role string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "awarenow",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func SetJWTSecret(secret string) {
	jwtSecret = []byte(secret)
}

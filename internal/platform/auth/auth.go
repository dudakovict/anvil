// Package auth provides support for issuing and verifying JWT access tokens.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

const key ctxKey = 0

type ctxKey int

type Config struct {
	Secret string
	TTL    time.Duration
}

type Claims struct {
	UserID int64
	Role   string
}

type jwtClaims struct {
	jwt.RegisteredClaims

	Role string `json:"role"`
}

type Auth struct {
	secret []byte
	ttl    time.Duration
}

func New(cfg Config) *Auth {
	return &Auth{secret: []byte(cfg.Secret), ttl: cfg.TTL}
}

func (a *Auth) TTL() time.Duration {
	return a.ttl
}

func (a *Auth) Generate(userID int64, role string) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
		},
		Role: role,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return token, nil
}

// Verify validates the token signature and expiry and returns the claims.
func (a *Auth) Verify(token string) (Claims, error) {
	var claims jwtClaims

	keyFunc := func(*jwt.Token) (any, error) { return a.secret, nil }

	if _, err := jwt.ParseWithClaims(
		token, &claims, keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	); err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: parsing subject: %w", ErrInvalidToken, err)
	}

	return Claims{UserID: id, Role: claims.Role}, nil
}

func WithContext(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, key, claims)
}

func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(key).(Claims)

	return claims, ok
}

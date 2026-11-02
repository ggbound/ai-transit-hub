package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateToken generates JWT token for user
func GenerateToken(user *models.User, cfg *config.Config) (string, error) {
	now := time.Now()
	expiryTime := now.Add(cfg.JWT.Expiry)

	logger.Info("Generating token for user: %s (ID: %d), expiry: %v, duration: %v",
		user.Email, user.ID, expiryTime, cfg.JWT.Expiry)

	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.App.Name,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.JWT.Secret))
	if err != nil {
		return "", err
	}

	logger.Debug("Token generated successfully, expires in: %v hours",
		cfg.JWT.Expiry.Hours())
	return signedToken, nil
}

// ValidateToken validates JWT token
func ValidateToken(tokenString string, cfg *config.Config) (*Claims, error) {

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWT.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			logger.Info("Token expired")
			return nil, ErrExpiredToken
		}
		logger.Error("Token validation error: %v", err)
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.ExpiresAt != nil {
			// now := time.Now()
			// expiresIn := claims.ExpiresAt.Time.Sub(now)
			// logger.Debug("Token valid, expires in: %v (at %v)",expiresIn, claims.ExpiresAt.Time)
		}
		return claims, nil
	}

	logger.Error("Invalid token format")
	return nil, ErrInvalidToken
}

// GetTokenExpiryInfo gets expiry info from token without validating signature
func GetTokenExpiryInfo(tokenString string) (time.Time, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return time.Time{}, err
	}

	if claims, ok := token.Claims.(*Claims); ok && claims.ExpiresAt != nil {
		return claims.ExpiresAt.Time, nil
	}

	return time.Time{}, errors.New("invalid token claims")
}

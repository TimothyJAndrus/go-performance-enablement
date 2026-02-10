package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/golang-jwt/jwt/v5"
	"github.com/wgu/go-performance-enablement/pkg/awsutils"
	"github.com/wgu/go-performance-enablement/pkg/metrics"
	"go.uber.org/zap"
)

var (
	logger        *zap.Logger
	awsClients    *awsutils.AWSClients
	currentRegion string
	jwtSecret     string
	jwtPublicKey  *rsa.PublicKey
	issuer        string
	audience      string
)

func init() {
	var err error

	// Initialize logger
	logger, _ = zap.NewProduction()

	// Get environment variables
	currentRegion = os.Getenv("AWS_REGION")
	issuer = os.Getenv("JWT_ISSUER")
	audience = os.Getenv("JWT_AUDIENCE")
	jwtSecretName := os.Getenv("JWT_SECRET_NAME")

	// Initialize AWS clients
	ctx := context.Background()
	awsClients, err = awsutils.NewAWSClients(ctx)
	if err != nil {
		logger.Fatal("failed to create AWS clients", zap.Error(err))
	}

	// Retrieve JWT secret from Secrets Manager
	if jwtSecretName != "" {
		jwtSecret, err = awsClients.GetSecret(ctx, jwtSecretName)
		if err != nil {
			logger.Fatal("failed to retrieve JWT secret", zap.Error(err))
		}
		logger.Info("JWT secret loaded from Secrets Manager")
	} else {
		// For local development
		jwtSecret = os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			logger.Warn("no JWT secret configured")
		}
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	TenantID string   `json:"tenant_id"`
	jwt.RegisteredClaims
}

// Handler authorizes API Gateway requests
func Handler(ctx context.Context, request events.APIGatewayCustomAuthorizerRequestTypeRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	start := time.Now()
	functionName := "authorizer"

	logger.Info("processing authorization request",
		zap.String("method", request.HTTPMethod),
		zap.String("path", request.Path),
	)

	// Extract token from Authorization header
	token := extractToken(request.Headers)
	if token == "" {
		logger.Warn("no authorization token provided")
		duration := time.Since(start)
		metrics.RecordLambdaInvocation(functionName, currentRegion, duration, errors.New("unauthorized"))
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	// Validate and parse JWT
	claims, err := validateToken(token)
	if err != nil {
		logger.Warn("token validation failed", zap.Error(err))
		duration := time.Since(start)
		metrics.RecordLambdaInvocation(functionName, currentRegion, duration, err)
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		logger.Warn("token expired",
			zap.String("user_id", claims.UserID),
			zap.Time("expired_at", claims.ExpiresAt.Time),
		)
		duration := time.Since(start)
		metrics.RecordLambdaInvocation(functionName, currentRegion, duration, errors.New("token_expired"))
		return generatePolicy(claims.UserID, "Deny", request.MethodArn), nil
	}

	// Check issuer
	if issuer != "" && claims.Issuer != issuer {
		logger.Warn("invalid issuer",
			zap.String("expected", issuer),
			zap.String("actual", claims.Issuer),
		)
		duration := time.Since(start)
		metrics.RecordLambdaInvocation(functionName, currentRegion, duration, errors.New("invalid_issuer"))
		return generatePolicy(claims.UserID, "Deny", request.MethodArn), nil
	}

	// Check audience
	if audience != "" && !contains(claims.Audience, audience) {
		logger.Warn("invalid audience",
			zap.String("expected", audience),
			zap.Strings("actual", claims.Audience),
		)
		duration := time.Since(start)
		metrics.RecordLambdaInvocation(functionName, currentRegion, duration, errors.New("invalid_audience"))
		return generatePolicy(claims.UserID, "Deny", request.MethodArn), nil
	}

	// Authorization successful
	duration := time.Since(start)
	metrics.RecordLambdaInvocation(functionName, currentRegion, duration, nil)

	logger.Info("authorization successful",
		zap.String("user_id", claims.UserID),
		zap.String("email", claims.Email),
		zap.Strings("roles", claims.Roles),
		zap.Duration("duration", duration),
	)

	// Generate allow policy with context
	policy := generatePolicy(claims.UserID, "Allow", request.MethodArn)
	policy.Context = map[string]interface{}{
		"userId":   claims.UserID,
		"email":    claims.Email,
		"roles":    strings.Join(claims.Roles, ","),
		"tenantId": claims.TenantID,
	}

	return policy, nil
}

// extractToken extracts JWT token from Authorization header
func extractToken(headers map[string]string) string {
	// Try Authorization header
	if auth, ok := headers["Authorization"]; ok {
		parts := strings.Split(auth, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Try lowercase authorization header
	if auth, ok := headers["authorization"]; ok {
		parts := strings.Split(auth, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	return ""
}

// validateToken validates and parses JWT token
func validateToken(tokenString string) (*Claims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() &&
			token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return appropriate key based on signing method
		if token.Method.Alg() == jwt.SigningMethodRS256.Alg() {
			if jwtPublicKey != nil {
				return jwtPublicKey, nil
			}
			return nil, errors.New("RSA public key not configured")
		}

		// Return HMAC secret
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// generatePolicy generates IAM policy for API Gateway
func generatePolicy(principalID, effect, resource string) events.APIGatewayCustomAuthorizerResponse {
	if principalID == "" {
		principalID = "unknown"
	}

	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: principalID,
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   effect,
					Resource: []string{resource},
				},
			},
		},
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// decodePublicKey decodes RSA public key from PEM format
func decodePublicKey(pemEncoded string) (*rsa.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(pemEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(decoded)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return publicKey, nil
}

func main() {
	lambda.Start(Handler)
}

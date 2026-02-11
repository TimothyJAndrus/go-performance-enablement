package main

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	// Initialize logger for tests
	logger, _ = zap.NewDevelopment()
	jwtSecret = "test-secret-key-for-testing-only"
	issuer = "test-issuer"
	audience = "test-audience"
	currentRegion = "us-west-2"
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name: "valid bearer token with Authorization header",
			headers: map[string]string{
				"Authorization": "Bearer test-token-123",
			},
			expected: "test-token-123",
		},
		{
			name: "valid bearer token with lowercase authorization header",
			headers: map[string]string{
				"authorization": "Bearer test-token-456",
			},
			expected: "test-token-456",
		},
		{
			name: "no authorization header",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expected: "",
		},
		{
			name: "invalid authorization format",
			headers: map[string]string{
				"Authorization": "test-token-789",
			},
			expected: "",
		},
		{
			name: "empty bearer token",
			headers: map[string]string{
				"Authorization": "Bearer ",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractToken(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateToken(t *testing.T) {
	// Create a valid token for testing
	validClaims := &Claims{
		UserID:   "user-123",
		Email:    "test@example.com",
		Roles:    []string{"user", "admin"},
		TenantID: "tenant-456",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user-123",
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
	validTokenString, _ := validToken.SignedString([]byte(jwtSecret))

	tests := []struct {
		name      string
		token     string
		wantErr   bool
		errString string
	}{
		{
			name:    "valid token",
			token:   validTokenString,
			wantErr: false,
		},
		{
			name:      "invalid token format",
			token:     "invalid.token.format",
			wantErr:   true,
			errString: "failed to parse token",
		},
		{
			name:      "empty token",
			token:     "",
			wantErr:   true,
			errString: "failed to parse token",
		},
		{
			name:      "token with wrong signature",
			token:     validTokenString + "x",
			wantErr:   true,
			errString: "failed to parse token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := validateToken(tt.token)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "user-123", claims.UserID)
				assert.Equal(t, "test@example.com", claims.Email)
				assert.Contains(t, claims.Roles, "user")
				assert.Contains(t, claims.Roles, "admin")
			}
		})
	}
}

func TestGeneratePolicy(t *testing.T) {
	tests := []struct {
		name        string
		principalID string
		effect      string
		resource    string
	}{
		{
			name:        "allow policy with principal",
			principalID: "user-123",
			effect:      "Allow",
			resource:    "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
		},
		{
			name:        "deny policy with principal",
			principalID: "user-456",
			effect:      "Deny",
			resource:    "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/POST/resource",
		},
		{
			name:        "empty principal defaults to unknown",
			principalID: "",
			effect:      "Deny",
			resource:    "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := generatePolicy(tt.principalID, tt.effect, tt.resource)

			expectedPrincipal := tt.principalID
			if expectedPrincipal == "" {
				expectedPrincipal = "unknown"
			}

			assert.Equal(t, expectedPrincipal, policy.PrincipalID)
			assert.Equal(t, "2012-10-17", policy.PolicyDocument.Version)
			assert.Len(t, policy.PolicyDocument.Statement, 1)
			assert.Equal(t, tt.effect, policy.PolicyDocument.Statement[0].Effect)
			assert.Contains(t, policy.PolicyDocument.Statement[0].Action, "execute-api:Invoke")
			assert.Contains(t, policy.PolicyDocument.Statement[0].Resource, tt.resource)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item present",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "item not present",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "grape",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "apple",
			expected: false,
		},
		{
			name:     "case sensitive",
			slice:    []string{"Apple", "Banana"},
			item:     "apple",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandler(t *testing.T) {
	// Create a valid token
	validClaims := &Claims{
		UserID:   "user-123",
		Email:    "test@example.com",
		Roles:    []string{"user", "admin"},
		TenantID: "tenant-456",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user-123",
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims)
	validTokenString, _ := validToken.SignedString([]byte(jwtSecret))

	// Create an expired token
	expiredClaims := &Claims{
		UserID: "user-789",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, _ := expiredToken.SignedString([]byte(jwtSecret))

	// Create token with wrong issuer
	wrongIssuerClaims := &Claims{
		UserID: "user-999",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "wrong-issuer",
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	wrongIssuerToken := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongIssuerClaims)
	wrongIssuerTokenString, _ := wrongIssuerToken.SignedString([]byte(jwtSecret))

	// Create token with wrong audience
	wrongAudienceClaims := &Claims{
		UserID: "user-888",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{"wrong-audience"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	wrongAudienceToken := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongAudienceClaims)
	wrongAudienceTokenString, _ := wrongAudienceToken.SignedString([]byte(jwtSecret))

	tests := []struct {
		name           string
		request        events.APIGatewayCustomAuthorizerRequestTypeRequest
		expectedEffect string
		expectContext  bool
	}{
		{
			name: "valid token - allow access",
			request: events.APIGatewayCustomAuthorizerRequestTypeRequest{
				HTTPMethod: "GET",
				Path:       "/api/resource",
				Headers: map[string]string{
					"Authorization": "Bearer " + validTokenString,
				},
				MethodArn: "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
			},
			expectedEffect: "Allow",
			expectContext:  true,
		},
		{
			name: "no token - deny access",
			request: events.APIGatewayCustomAuthorizerRequestTypeRequest{
				HTTPMethod: "GET",
				Path:       "/api/resource",
				Headers:    map[string]string{},
				MethodArn:  "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
			},
			expectedEffect: "Deny",
			expectContext:  false,
		},
		{
			name: "expired token - deny access",
			request: events.APIGatewayCustomAuthorizerRequestTypeRequest{
				HTTPMethod: "GET",
				Path:       "/api/resource",
				Headers: map[string]string{
					"Authorization": "Bearer " + expiredTokenString,
				},
				MethodArn: "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
			},
			expectedEffect: "Deny",
			expectContext:  false,
		},
		{
			name: "invalid token - deny access",
			request: events.APIGatewayCustomAuthorizerRequestTypeRequest{
				HTTPMethod: "GET",
				Path:       "/api/resource",
				Headers: map[string]string{
					"Authorization": "Bearer invalid.token.string",
				},
				MethodArn: "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
			},
			expectedEffect: "Deny",
			expectContext:  false,
		},
		{
			name: "wrong issuer - deny access",
			request: events.APIGatewayCustomAuthorizerRequestTypeRequest{
				HTTPMethod: "GET",
				Path:       "/api/resource",
				Headers: map[string]string{
					"Authorization": "Bearer " + wrongIssuerTokenString,
				},
				MethodArn: "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
			},
			expectedEffect: "Deny",
			expectContext:  false,
		},
		{
			name: "wrong audience - deny access",
			request: events.APIGatewayCustomAuthorizerRequestTypeRequest{
				HTTPMethod: "GET",
				Path:       "/api/resource",
				Headers: map[string]string{
					"Authorization": "Bearer " + wrongAudienceTokenString,
				},
				MethodArn: "arn:aws:execute-api:us-west-2:123456789012:api-id/stage/GET/resource",
			},
			expectedEffect: "Deny",
			expectContext:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			response, err := Handler(ctx, tt.request)

			assert.NoError(t, err)
			assert.NotEmpty(t, response.PrincipalID)
			assert.Equal(t, tt.expectedEffect, response.PolicyDocument.Statement[0].Effect)

			if tt.expectContext {
				assert.NotEmpty(t, response.Context)
				assert.Contains(t, response.Context, "userId")
				assert.Contains(t, response.Context, "email")
				assert.Contains(t, response.Context, "roles")
				assert.Equal(t, "user-123", response.Context["userId"])
				assert.Equal(t, "test@example.com", response.Context["email"])
			} else {
				// Deny responses typically don't have context or have empty context
				if response.Context != nil {
					assert.Empty(t, response.Context)
				}
			}
		})
	}
}

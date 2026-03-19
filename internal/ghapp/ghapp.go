package ghapp

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v67/github"
)

// Client wraps GitHub App authentication, providing installation-scoped tokens.
type Client struct {
	appID      int64
	privateKey *rsa.PrivateKey

	mu           sync.Mutex
	tokenCache   map[int64]cachedToken
}

type cachedToken struct {
	token   string
	expires time.Time
}

// New creates a GitHub App client from app ID and PEM-encoded private key.
func New(appID int64, pemKey []byte) (*Client, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		k, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		var ok bool
		key, ok = k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}
	return &Client{appID: appID, privateKey: key, tokenCache: make(map[int64]cachedToken)}, nil
}

// createJWT creates a short-lived JWT for GitHub App authentication.
func (c *Client) createJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    fmt.Sprintf("%d", c.appID),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(c.privateKey)
}

// appClient returns a GitHub client authenticated as the App (not an installation).
func (c *Client) appClient() (*github.Client, error) {
	jwtToken, err := c.createJWT()
	if err != nil {
		return nil, err
	}
	return github.NewClient(nil).WithAuthToken(jwtToken), nil
}

// InstallationClient returns a GitHub client scoped to a specific installation.
func (c *Client) InstallationClient(ctx context.Context, installationID int64) (*github.Client, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	return github.NewClient(nil).WithAuthToken(token), nil
}

func (c *Client) installationToken(ctx context.Context, installationID int64) (string, error) {
	c.mu.Lock()
	if cached, ok := c.tokenCache[installationID]; ok && time.Now().Before(cached.expires) {
		c.mu.Unlock()
		return cached.token, nil
	}
	c.mu.Unlock()

	gh, err := c.appClient()
	if err != nil {
		return "", err
	}

	token, _, err := gh.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return "", fmt.Errorf("create installation token: %w", err)
	}

	c.mu.Lock()
	c.tokenCache[installationID] = cachedToken{
		token:   token.GetToken(),
		expires: token.GetExpiresAt().Time.Add(-5 * time.Minute), // refresh 5min early
	}
	c.mu.Unlock()

	return token.GetToken(), nil
}

// FindInstallation finds the installation ID for a given repository.
func (c *Client) FindInstallation(ctx context.Context, owner, repo string) (int64, error) {
	gh, err := c.appClient()
	if err != nil {
		return 0, err
	}
	install, _, err := gh.Apps.FindRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return 0, fmt.Errorf("find installation for %s/%s: %w", owner, repo, err)
	}
	return install.GetID(), nil
}

// WebhookMiddleware validates GitHub webhook signatures using the webhook secret.
func WebhookMiddleware(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			next.ServeHTTP(w, r)
			return
		}
		payload, err := github.ValidatePayload(r, []byte(secret))
		if err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		r.Body = http.NoBody // already consumed
		ctx := context.WithValue(r.Context(), payloadKey, payload)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextKey string

const payloadKey contextKey = "webhook_payload"

// PayloadFromContext retrieves the validated webhook payload from context.
func PayloadFromContext(ctx context.Context) []byte {
	if v, ok := ctx.Value(payloadKey).([]byte); ok {
		return v
	}
	return nil
}

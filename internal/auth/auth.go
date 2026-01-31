package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const defaultBaseURL = "https://api.github.com"

type options struct {
	baseURL string
}

// Option configures auth behaviour.
type Option func(*options)

// WithBaseURL overrides the GitHub API base URL (used for testing).
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

func buildOpts(opts []Option) options {
	o := options{baseURL: defaultBaseURL}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// GenerateJWT creates a JWT signed with the GitHub App's RSA private key.
func GenerateJWT(appID int64, privateKeyPath string) (string, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("reading private key %s: %w", privateKeyPath, err)
	}

	key, err := findRSAKey(keyData)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-30 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    strconv.FormatInt(appID, 10),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signed, nil
}

var keyBlockTypes = map[string]bool{
	"RSA PRIVATE KEY": true,
	"PRIVATE KEY":     true,
}

func findRSAKey(pemData []byte) (*rsa.PrivateKey, error) {
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("no RSA private key PEM block found")
		}
		if keyBlockTypes[block.Type] {
			return parsePKCS1OrPKCS8(block.Bytes)
		}
	}
}

func parsePKCS1OrPKCS8(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}

	pkcs8Key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parsing private key (tried PKCS1 and PKCS8): %w", err)
	}

	key, ok := pkcs8Key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("PKCS8 key is not RSA")
	}
	return key, nil
}

type installationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

const maxResponseBytes = 1 << 20

// GetInstallationToken exchanges a JWT for a GitHub App installation access token.
func GetInstallationToken(jwtToken string, installationID int64, opts ...Option) (string, error) {
	o := buildOpts(opts)

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", o.baseURL, installationID)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting installation token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp installationTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("GitHub API returned empty token")
	}

	return tokenResp.Token, nil
}

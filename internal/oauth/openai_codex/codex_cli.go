package openai_codex

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/marang/franz-agent/internal/home"
	"github.com/marang/franz-agent/internal/oauth"
)

var ErrCodexCLIAuthNotFound = errors.New("codex cli auth not found")

type CodexCLIAuth struct {
	Token     *oauth.Token
	AccountID string
	PlanType  string
	Path      string
}

type codexAuthFile struct {
	OpenAIAPIKey string           `json:"OPENAI_API_KEY"`
	Tokens       *codexAuthTokens `json:"tokens"`
}

type codexAuthTokens struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id"`
}

type codexAuthJWTClaims struct {
	Auth struct {
		ChatGPTPlanType  string `json:"chatgpt_plan_type"`
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"https://api.openai.com/auth"`
	Exp int64 `json:"exp"`
}

func ReadCodexCLIAuth() (*CodexCLIAuth, error) {
	var readErrs []error
	for _, path := range codexCLIAuthPaths() {
		auth, err := readCodexCLIAuthFile(path)
		if err == nil {
			return auth, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			readErrs = append(readErrs, err)
		}
	}
	if len(readErrs) > 0 {
		return nil, errors.Join(readErrs...)
	}
	return nil, ErrCodexCLIAuthNotFound
}

func codexCLIAuthPaths() []string {
	var paths []string
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		paths = append(paths, filepath.Join(codexHome, "auth.json"))
	}
	paths = append(paths, filepath.Join(home.Dir(), ".codex", "auth.json"))
	return paths
}

func readCodexCLIAuthFile(path string) (*CodexCLIAuth, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var payload codexAuthFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if payload.Tokens == nil || strings.TrimSpace(payload.Tokens.AccessToken) == "" {
		return nil, fmt.Errorf("%s does not contain ChatGPT Codex tokens", path)
	}

	idClaims, _ := parseCodexAuthJWT(payload.Tokens.IDToken)
	accessClaims, _ := parseCodexAuthJWT(payload.Tokens.AccessToken)
	accountID := firstNonEmpty(
		payload.Tokens.AccountID,
		idClaims.Auth.ChatGPTAccountID,
		accessClaims.Auth.ChatGPTAccountID,
	)
	if strings.TrimSpace(accountID) == "" {
		if parsed, err := ExtractAccountID(payload.Tokens.AccessToken); err == nil {
			accountID = parsed
		}
	}
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("%s does not contain a ChatGPT account id", path)
	}

	token := &oauth.Token{
		AccessToken:  strings.TrimSpace(payload.Tokens.AccessToken),
		RefreshToken: strings.TrimSpace(payload.Tokens.RefreshToken),
	}
	if accessClaims.Exp > 0 {
		token.ExpiresAt = accessClaims.Exp
		token.SetExpiresIn()
	}

	return &CodexCLIAuth{
		Token:     token,
		AccountID: strings.TrimSpace(accountID),
		PlanType:  firstNonEmpty(idClaims.Auth.ChatGPTPlanType, accessClaims.Auth.ChatGPTPlanType),
		Path:      path,
	}, nil
}

func parseCodexAuthJWT(raw string) (codexAuthJWTClaims, error) {
	var claims codexAuthJWTClaims
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return claims, fmt.Errorf("invalid JWT format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, fmt.Errorf("decode JWT payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return claims, fmt.Errorf("parse JWT payload: %w", err)
	}
	return claims, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

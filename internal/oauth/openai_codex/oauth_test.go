package openai_codex

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseAuthorizationInput(t *testing.T) {
	t.Parallel()

	code, state := ParseAuthorizationInput("http://localhost:1455/auth/callback?code=abc&state=xyz")
	require.Equal(t, "abc", code)
	require.Equal(t, "xyz", state)

	code, state = ParseAuthorizationInput("abc#xyz")
	require.Equal(t, "abc", code)
	require.Equal(t, "xyz", state)

	code, state = ParseAuthorizationInput("code=123&state=456")
	require.Equal(t, "123", code)
	require.Equal(t, "456", state)
}

func TestExtractAccountID(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		jwtClaimKey: map[string]any{
			"chatgpt_account_id": "acc_123",
		},
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	token := "h." + base64.RawURLEncoding.EncodeToString(payloadJSON) + ".s"
	accountID, err := ExtractAccountID(token)
	require.NoError(t, err)
	require.Equal(t, "acc_123", accountID)
}

func TestReadCodexCLIAuth(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	expiresAt := time.Now().Add(time.Hour).Unix()
	idToken := testJWT(t, map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc_from_id",
			"chatgpt_plan_type":  "pro",
		},
	})
	accessToken := testJWT(t, map[string]any{
		"exp": expiresAt,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acc_from_access",
		},
	})
	authJSON := map[string]any{
		"tokens": map[string]any{
			"id_token":      idToken,
			"access_token":  accessToken,
			"refresh_token": "refresh-token",
		},
	}
	data, err := json.Marshal(authJSON)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(codexHome, "auth.json"), data, 0o600))

	auth, err := ReadCodexCLIAuth()
	require.NoError(t, err)
	require.Equal(t, "acc_from_id", auth.AccountID)
	require.Equal(t, "pro", auth.PlanType)
	require.Equal(t, accessToken, auth.Token.AccessToken)
	require.Equal(t, "refresh-token", auth.Token.RefreshToken)
	require.Equal(t, expiresAt, auth.Token.ExpiresAt)
	require.Equal(t, filepath.Join(codexHome, "auth.json"), auth.Path)
}

func testJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	require.NoError(t, err)
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + "."
}

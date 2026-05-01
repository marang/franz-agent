package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marang/franz-agent/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseScopeFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    config.Scope
		wantErr bool
	}{
		{name: "global", value: "global", want: config.ScopeGlobal},
		{name: "workspace", value: "workspace", want: config.ScopeWorkspace},
		{name: "uppercase", value: "GLOBAL", want: config.ScopeGlobal},
		{name: "invalid", value: "project", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &cobra.Command{}
			c.Flags().String(scopeFlagName, "", "")
			require.NoError(t, c.Flags().Set(scopeFlagName, tt.value))

			got, err := parseScopeFlag(c)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeSkillPath(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if vol := filepath.VolumeName(home); vol != "" {
		t.Setenv("HOMEDRIVE", vol)
		t.Setenv("HOMEPATH", strings.TrimPrefix(home, vol))
	}

	got, err := normalizeSkillPath(cwd, "skills")
	require.NoError(t, err)
	require.Equal(t, cwd+string(os.PathSeparator)+"skills", got)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	got, err = normalizeSkillPath(cwd, "~/skills")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(homeDir, "skills"), got)

	got, err = normalizeSkillPath(cwd, "")
	require.Error(t, err)
	require.Empty(t, got)
}

func TestSkillsSHSearchResultInstallSource(t *testing.T) {
	t.Parallel()

	result := skillsSHSearchResult{Source: "owner/repo", Slug: "my-skill"}
	require.Equal(t, "skills.sh/owner/repo/my-skill", result.InstallSource())

	result = skillsSHSearchResult{Source: "owner/repo", Name: "fallback-skill"}
	require.Equal(t, "skills.sh/owner/repo/fallback-skill", result.InstallSource())

	result = skillsSHSearchResult{Source: "owner/repo"}
	require.Equal(t, "skills.sh/owner/repo", result.InstallSource())
}

func TestSearchSkillsSH(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/search", r.URL.Path)
		require.Equal(t, "go", r.URL.Query().Get("q"))
		require.Equal(t, "go", r.URL.Query().Get("term"))
		require.Equal(t, "10", r.URL.Query().Get("limit"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode([]skillsSHSearchResult{
			{Name: "beta", Source: "o/r", Slug: "beta", Installs: 2},
			{Name: "alpha", Source: "o/r", Slug: "alpha", Installs: 5},
		}))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Setenv("SKILLS_API_URL", server.URL)

	results, err := searchSkillsSH(context.Background(), "go", 10)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "alpha", results[0].Name)
	require.Equal(t, "beta", results[1].Name)
}

func TestSearchSkillsSHWrappedResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/search", r.URL.Path)
		require.Equal(t, "go", r.URL.Query().Get("q"))
		require.Equal(t, "go", r.URL.Query().Get("term"))
		require.Equal(t, "10", r.URL.Query().Get("limit"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"query": "go",
			"skills": []map[string]any{
				{"name": "beta", "source": "o/r", "skillId": "beta", "installs": 2},
				{"name": "alpha", "source": "o/r", "skillId": "alpha", "installs": 5},
			},
		}))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Setenv("SKILLS_API_URL", server.URL)

	results, err := searchSkillsSH(context.Background(), "go", 10)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "alpha", results[0].Name)
	require.Equal(t, "skills.sh/o/r/alpha", results[0].InstallSource())
	require.Equal(t, "beta", results[1].Name)
}

func TestSearchSkillsSHNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte("upstream error"))
		require.NoError(t, err)
	}))
	defer server.Close()

	t.Setenv("SKILLS_API_URL", server.URL)

	_, err := searchSkillsSH(context.Background(), "go", 10)
	require.Error(t, err)
	require.Contains(t, err.Error(), "skills.sh search failed: upstream error")
}

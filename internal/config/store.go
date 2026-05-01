package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"charm.land/catwalk/pkg/catwalk"
	hyperp "github.com/marang/franz-agent/internal/agent/hyper"
	"github.com/marang/franz-agent/internal/oauth"
	"github.com/marang/franz-agent/internal/oauth/copilot"
	"github.com/marang/franz-agent/internal/oauth/hyper"
	"github.com/marang/franz-agent/internal/oauth/openai_codex"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConfigStore is the single entry point for all config access. It owns the
// pure-data Config, runtime state (working directory, resolver, known
// providers), and persistence to both global and workspace config files.
type ConfigStore struct {
	config         *Config
	workingDir     string
	resolver       VariableResolver
	globalDataPath string // ~/.local/share/franz-agent/franz-agent.json
	workspacePath  string // .franz-agent/franz-agent.json
	knownProviders []catwalk.Provider
	configMu       sync.Mutex
}

// Config returns the pure-data config struct (read-only after load).
func (s *ConfigStore) Config() *Config {
	return s.config
}

// WorkingDir returns the current working directory.
func (s *ConfigStore) WorkingDir() string {
	return s.workingDir
}

// Resolver returns the variable resolver.
func (s *ConfigStore) Resolver() VariableResolver {
	return s.resolver
}

// Resolve resolves a variable reference using the configured resolver.
func (s *ConfigStore) Resolve(key string) (string, error) {
	if s.resolver == nil {
		return "", fmt.Errorf("no variable resolver configured")
	}
	return s.resolver.ResolveValue(key)
}

// KnownProviders returns the list of known providers.
func (s *ConfigStore) KnownProviders() []catwalk.Provider {
	return s.knownProviders
}

// SetupAgents configures the coder and task agents on the config.
func (s *ConfigStore) SetupAgents() {
	s.config.SetupAgents()
}

// configPath returns the file path for the given scope.
func (s *ConfigStore) configPath(scope Scope) string {
	switch scope {
	case ScopeWorkspace:
		return s.workspacePath
	default:
		return s.globalDataPath
	}
}

// HasConfigField checks whether a key exists in the config file for the given
// scope.
func (s *ConfigStore) HasConfigField(scope Scope, key string) bool {
	data, err := os.ReadFile(s.configPath(scope))
	if err != nil {
		return false
	}
	return gjson.Get(string(data), key).Exists()
}

// SetConfigField sets a key/value pair in the config file for the given scope.
func (s *ConfigStore) SetConfigField(scope Scope, key string, value any) error {
	return s.SetConfigFields(scope, map[string]any{key: value})
}

// SetConfigFields sets multiple key/value pairs in one config file update.
func (s *ConfigStore) SetConfigFields(scope Scope, fields map[string]any) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	path := s.configPath(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	newValue := string(data)
	for key, value := range fields {
		newValue, err = sjson.Set(newValue, key, value)
		if err != nil {
			return fmt.Errorf("failed to set config field %s: %w", key, err)
		}
	}
	return writeConfigFile(path, []byte(newValue))
}

// RemoveConfigField removes a key from the config file for the given scope.
func (s *ConfigStore) RemoveConfigField(scope Scope, key string) error {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	path := s.configPath(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	newValue, err := sjson.Delete(string(data), key)
	if err != nil {
		return fmt.Errorf("failed to delete config field %s: %w", key, err)
	}
	return writeConfigFile(path, []byte(newValue))
}

func writeConfigFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", path, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to set temporary config permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	return nil
}

// SkillsPaths returns the configured skills paths from the config file for the
// given scope. It only returns explicitly configured paths and does not include
// default or derived paths added at runtime.
func (s *ConfigStore) SkillsPaths(scope Scope) ([]string, error) {
	path := s.configPath(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	value := gjson.GetBytes(data, "options.skills_paths")
	if !value.Exists() || !value.IsArray() {
		return []string{}, nil
	}

	paths := make([]string, 0, len(value.Array()))
	for _, item := range value.Array() {
		if item.Type == gjson.String {
			paths = append(paths, item.String())
		}
	}

	return paths, nil
}

// UpdatePreferredModel updates the preferred model for the given type and
// persists it to the config file at the given scope.
func (s *ConfigStore) UpdatePreferredModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	s.config.Models[modelType] = model
	if err := s.SetConfigField(scope, fmt.Sprintf("models.%s", modelType), model); err != nil {
		return fmt.Errorf("failed to update preferred model: %w", err)
	}
	if err := s.recordRecentModel(scope, modelType, model); err != nil {
		return err
	}
	return nil
}

// SetCompactMode sets the compact mode setting and persists it.
func (s *ConfigStore) SetCompactMode(scope Scope, enabled bool) error {
	if s.config.Options == nil {
		s.config.Options = &Options{}
	}
	s.config.Options.TUI.CompactMode = enabled
	return s.SetConfigField(scope, "options.tui.compact_mode", enabled)
}

// SetTransparentBackground sets the transparent background setting and persists it.
func (s *ConfigStore) SetTransparentBackground(scope Scope, enabled bool) error {
	if s.config.Options == nil {
		s.config.Options = &Options{}
	}
	s.config.Options.TUI.Transparent = &enabled
	return s.SetConfigField(scope, "options.tui.transparent", enabled)
}

// SetProviderAPIKey sets the API key for a provider and persists it.
func (s *ConfigStore) SetProviderAPIKey(scope Scope, providerID string, apiKey any) error {
	var providerConfig ProviderConfig
	var exists bool
	var setKeyOrToken func()

	switch v := apiKey.(type) {
	case string:
		setKeyOrToken = func() { providerConfig.APIKey = v }
	case *oauth.Token:
		accountID := ""
		if providerID == openai_codex.ProviderID {
			var err error
			accountID, err = openai_codex.ExtractAccountID(v.AccessToken)
			if err != nil {
				return fmt.Errorf("failed to extract chatgpt account id from access token: %w", err)
			}
		}
		setKeyOrToken = func() {
			providerConfig.APIKey = v.AccessToken
			providerConfig.OAuthToken = v
			switch providerID {
			case string(catwalk.InferenceProviderCopilot):
				providerConfig.SetupGitHubCopilot()
			case openai_codex.ProviderID:
				providerConfig.SetupOpenAICodex(accountID)
				providerConfig.Models = openAICodexModelsForProvider(providerConfig, providerConfig.Models)
			}
		}
	}

	providerConfig, exists = s.config.Providers.Get(providerID)
	if exists {
		setKeyOrToken()
		s.config.Providers.Set(providerID, providerConfig)
		return s.persistProviderCredential(scope, providerID, providerConfig)
	}

	var foundProvider *catwalk.Provider
	for _, p := range s.knownProviders {
		if string(p.ID) == providerID {
			foundProvider = &p
			break
		}
	}

	if foundProvider != nil {
		providerConfig = ProviderConfig{
			ID:           providerID,
			Name:         foundProvider.Name,
			BaseURL:      foundProvider.APIEndpoint,
			Type:         foundProvider.Type,
			Disable:      false,
			ExtraHeaders: make(map[string]string),
			ExtraParams:  make(map[string]string),
			Models:       foundProvider.Models,
		}
		setKeyOrToken()
	} else {
		return fmt.Errorf("provider with ID %s not found in known providers", providerID)
	}
	s.config.Providers.Set(providerID, providerConfig)

	return s.persistProviderCredential(scope, providerID, providerConfig)
}

func (s *ConfigStore) persistProviderCredential(scope Scope, providerID string, providerConfig ProviderConfig) error {
	fields := map[string]any{
		fmt.Sprintf("providers.%s.api_key", providerID): providerConfig.APIKey,
	}
	if providerConfig.OAuthToken != nil {
		fields[fmt.Sprintf("providers.%s.oauth", providerID)] = providerConfig.OAuthToken
	}
	if providerID == openai_codex.ProviderID {
		fields[fmt.Sprintf("providers.%s.extra_headers", providerID)] = providerConfig.ExtraHeaders
	}
	if err := s.SetConfigFields(scope, fields); err != nil {
		return fmt.Errorf("failed to save provider credentials to config file: %w", err)
	}
	return nil
}

// RefreshOAuthToken refreshes the OAuth token for the given provider.
func (s *ConfigStore) RefreshOAuthToken(ctx context.Context, scope Scope, providerID string) error {
	providerConfig, exists := s.config.Providers.Get(providerID)
	if !exists {
		return fmt.Errorf("provider %s not found", providerID)
	}

	if providerConfig.OAuthToken == nil {
		return fmt.Errorf("provider %s does not have an OAuth token", providerID)
	}

	var newToken *oauth.Token
	var refreshErr error
	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		newToken, refreshErr = copilot.RefreshToken(ctx, providerConfig.OAuthToken.RefreshToken)
	case hyperp.Name:
		newToken, refreshErr = hyper.ExchangeToken(ctx, providerConfig.OAuthToken.RefreshToken)
	case openai_codex.ProviderID:
		newToken, refreshErr = openai_codex.RefreshToken(ctx, providerConfig.OAuthToken.RefreshToken)
	default:
		return fmt.Errorf("OAuth refresh not supported for provider %s", providerID)
	}
	if refreshErr != nil {
		return fmt.Errorf("failed to refresh OAuth token for provider %s: %w", providerID, refreshErr)
	}

	slog.Info("Successfully refreshed OAuth token", "provider", providerID)
	providerConfig.OAuthToken = newToken
	providerConfig.APIKey = newToken.AccessToken

	switch providerID {
	case string(catwalk.InferenceProviderCopilot):
		providerConfig.SetupGitHubCopilot()
	case openai_codex.ProviderID:
		accountID, err := openai_codex.ExtractAccountID(newToken.AccessToken)
		if err != nil {
			return fmt.Errorf("failed to extract chatgpt account id from refreshed token: %w", err)
		}
		providerConfig.SetupOpenAICodex(accountID)
		providerConfig.Models = openAICodexModelsForProvider(providerConfig, providerConfig.Models)
	}

	s.config.Providers.Set(providerID, providerConfig)

	fields := map[string]any{
		fmt.Sprintf("providers.%s.api_key", providerID): newToken.AccessToken,
		fmt.Sprintf("providers.%s.oauth", providerID):   newToken,
	}
	if providerID == openai_codex.ProviderID {
		fields[fmt.Sprintf("providers.%s.extra_headers", providerID)] = providerConfig.ExtraHeaders
	}
	if err := s.SetConfigFields(scope, fields); err != nil {
		return fmt.Errorf("failed to persist refreshed token: %w", err)
	}

	return nil
}

// recordRecentModel records a model in the recent models list.
func (s *ConfigStore) recordRecentModel(scope Scope, modelType SelectedModelType, model SelectedModel) error {
	if model.Provider == "" || model.Model == "" {
		return nil
	}

	if s.config.RecentModels == nil {
		s.config.RecentModels = make(map[SelectedModelType][]SelectedModel)
	}

	eq := func(a, b SelectedModel) bool {
		return a.Provider == b.Provider && a.Model == b.Model
	}

	entry := SelectedModel{
		Provider: model.Provider,
		Model:    model.Model,
	}

	current := s.config.RecentModels[modelType]
	withoutCurrent := slices.DeleteFunc(slices.Clone(current), func(existing SelectedModel) bool {
		return eq(existing, entry)
	})

	updated := append([]SelectedModel{entry}, withoutCurrent...)
	if len(updated) > maxRecentModelsPerType {
		updated = updated[:maxRecentModelsPerType]
	}

	if slices.EqualFunc(current, updated, eq) {
		return nil
	}

	s.config.RecentModels[modelType] = updated

	if err := s.SetConfigField(scope, fmt.Sprintf("recent_models.%s", modelType), updated); err != nil {
		return fmt.Errorf("failed to persist recent models: %w", err)
	}

	return nil
}

// ImportCopilot attempts to import a GitHub Copilot token from disk.
func (s *ConfigStore) ImportCopilot() (*oauth.Token, bool) {
	if s.HasConfigField(ScopeGlobal, "providers.copilot.api_key") || s.HasConfigField(ScopeGlobal, "providers.copilot.oauth") {
		return nil, false
	}

	diskToken, hasDiskToken := copilot.RefreshTokenFromDisk()
	if !hasDiskToken {
		return nil, false
	}

	slog.Info("Found existing GitHub Copilot token on disk. Authenticating...")
	token, err := copilot.RefreshToken(context.TODO(), diskToken)
	if err != nil {
		slog.Error("Unable to import GitHub Copilot token", "error", err)
		return nil, false
	}

	if err := s.SetProviderAPIKey(ScopeGlobal, string(catwalk.InferenceProviderCopilot), token); err != nil {
		return token, false
	}

	slog.Info("GitHub Copilot successfully imported")
	return token, true
}

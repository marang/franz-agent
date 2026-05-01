package config

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
	"github.com/charmbracelet/x/etag"
	"github.com/marang/franz-agent/internal/agent/hyper"
	"github.com/marang/franz-agent/internal/csync"
	"github.com/marang/franz-agent/internal/home"
)

type syncer[T any] interface {
	Get(context.Context) (T, error)
}

var (
	providerCacheMu sync.Mutex
	providerCache   = make(map[providerCacheKey]*providerCacheEntry)
)

type providerCacheKey struct {
	autoupdate          bool
	customProvidersOnly bool
	catwalkURL          string
	hyperURL            string
	providersPath       string
	hyperPath           string
}

type providerCacheEntry struct {
	once sync.Once
	list []catwalk.Provider
	err  error
}

// file to cache provider data
func cachePathFor(name string) string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName, name+".json")
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/franz/`
	// for linux and macOS, it should be in `$HOME/.local/share/franz/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, name+".json")
	}

	return filepath.Join(home.Dir(), ".local", "share", appName, name+".json")
}

// UpdateProviders updates the Catwalk providers list from a specified source.
func UpdateProviders(pathOrURL string) error {
	var providers []catwalk.Provider
	pathOrURL = cmp.Or(pathOrURL, os.Getenv("CATWALK_URL"), defaultCatwalkURL)

	switch {
	case pathOrURL == "embedded":
		providers = embedded.GetAll()
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		var err error
		providers, err = catwalk.NewWithURL(pathOrURL).GetProviders(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch providers from Catwalk: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &providers); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
		if len(providers) == 0 {
			return fmt.Errorf("no providers found in the provided source")
		}
	}

	if err := newCache[[]catwalk.Provider](cachePathFor("providers")).Store(providers); err != nil {
		return fmt.Errorf("failed to save providers to cache: %w", err)
	}

	slog.Info("Providers updated successfully", "count", len(providers), "from", pathOrURL, "to", cachePathFor)
	return nil
}

// UpdateHyper updates the Hyper provider information from a specified URL.
func UpdateHyper(pathOrURL string) error {
	if !hyper.Enabled() {
		return fmt.Errorf("hyper not enabled")
	}
	var provider catwalk.Provider
	pathOrURL = cmp.Or(pathOrURL, hyper.BaseURL())

	switch {
	case pathOrURL == "embedded":
		provider = hyper.Embedded()
	case strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://"):
		client := realHyperClient{baseURL: pathOrURL}
		var err error
		provider, err = client.Get(context.Background(), "")
		if err != nil {
			return fmt.Errorf("failed to fetch provider from Hyper: %w", err)
		}
	default:
		content, err := os.ReadFile(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(content, &provider); err != nil {
			return fmt.Errorf("failed to unmarshal provider data: %w", err)
		}
	}

	if err := newCache[catwalk.Provider](cachePathFor("hyper")).Store(provider); err != nil {
		return fmt.Errorf("failed to save Hyper provider to cache: %w", err)
	}

	slog.Info("Hyper provider updated successfully", "from", pathOrURL, "to", cachePathFor("hyper"))
	return nil
}

var (
	catwalkSyncer = &catwalkSync{}
	hyperSyncer   = &hyperSync{}
	syncerMu      sync.Mutex
)

// Providers returns the list of providers, taking into account cached results
// and whether or not auto update is enabled.
//
// It will:
// 1. if auto update is disabled, it'll return the embedded providers at the
// time of release.
// 2. load the cached providers
// 3. try to get the fresh list of providers, and return either this new list,
// the cached list, or the embedded list if all others fail.
func Providers(cfg *Config) ([]catwalk.Provider, error) {
	key := providerKey(cfg)
	entry := providerEntry(key)
	entry.once.Do(func() {
		entry.list, entry.err = loadProviders(key)
	})
	return slices.Clone(entry.list), entry.err
}

func providerKey(cfg *Config) providerCacheKey {
	var options Options
	if cfg != nil && cfg.Options != nil {
		options = *cfg.Options
	}
	return providerCacheKey{
		autoupdate:          !options.DisableProviderAutoUpdate,
		customProvidersOnly: options.DisableDefaultProviders,
		catwalkURL:          cmp.Or(os.Getenv("CATWALK_URL"), defaultCatwalkURL),
		hyperURL:            hyper.BaseURL(),
		providersPath:       cachePathFor("providers"),
		hyperPath:           cachePathFor("hyper"),
	}
}

func providerEntry(key providerCacheKey) *providerCacheEntry {
	providerCacheMu.Lock()
	defer providerCacheMu.Unlock()

	entry := providerCache[key]
	if entry == nil {
		entry = &providerCacheEntry{}
		providerCache[key] = entry
	}
	return entry
}

func loadProviders(key providerCacheKey) ([]catwalk.Provider, error) {
	var wg sync.WaitGroup
	providers := csync.NewSlice[catwalk.Provider]()
	errs := make(chan error, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	wg.Go(func() {
		if key.customProvidersOnly {
			return
		}
		client := catwalk.NewWithURL(key.catwalkURL)
		syncerMu.Lock()
		defer syncerMu.Unlock()
		catwalkSyncer.Init(client, key.providersPath, key.autoupdate)

		items, err := catwalkSyncer.Get(ctx)
		if err != nil {
			catwalkURL := fmt.Sprintf("%s/v2/providers", key.catwalkURL)
			errs <- fmt.Errorf("Franz was unable to fetch an updated list of providers from %s. Consider setting FRANZ_DISABLE_PROVIDER_AUTO_UPDATE=1 to use the embedded providers bundled at the time of this Franz release. You can also update providers manually. For more info see franz-agent update-providers --help.\n\nCause: %w", catwalkURL, err) //nolint:staticcheck
			return
		}
		providers.Append(items...)
	})

	wg.Go(func() {
		if key.customProvidersOnly || !hyper.Enabled() {
			return
		}
		syncerMu.Lock()
		defer syncerMu.Unlock()
		hyperSyncer.Init(realHyperClient{baseURL: key.hyperURL}, key.hyperPath, key.autoupdate)

		item, err := hyperSyncer.Get(ctx)
		if err != nil {
			errs <- fmt.Errorf("Franz was unable to fetch updated information from Hyper: %w", err) //nolint:staticcheck
			return
		}
		providers.Append(item)
	})

	wg.Wait()
	close(errs)

	list := slices.Collect(providers.Seq())
	if !key.customProvidersOnly {
		list = ensureOpenAICodexProvider(list)
	}

	collectedErrs := make([]error, 0, len(errs))
	for err := range errs {
		collectedErrs = append(collectedErrs, err)
	}
	return list, errors.Join(collectedErrs...)
}

type cache[T any] struct {
	path string
}

func newCache[T any](path string) cache[T] {
	return cache[T]{path: path}
}

func (c cache[T]) Get() (T, string, error) {
	var v T
	data, err := os.ReadFile(c.path)
	if err != nil {
		return v, "", fmt.Errorf("failed to read provider cache file: %w", err)
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return v, "", fmt.Errorf("failed to unmarshal provider data from cache: %w", err)
	}

	return v, etag.Of(data), nil
}

func (c cache[T]) Store(v T) error {
	slog.Info("Saving provider data to disk", "path", c.path)
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for provider cache: %w", err)
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal provider data: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write provider data to cache: %w", err)
	}
	return nil
}

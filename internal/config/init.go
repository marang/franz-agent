package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/marang/franz-agent/internal/fsext"
	"github.com/marang/franz-agent/internal/home"
)

const (
	InitFlagFilename = "init"
)

type ProjectInitFlag struct {
	Initialized bool `json:"initialized"`
}

func Init(workingDir, dataDir string, debug bool) (*ConfigStore, error) {
	store, err := Load(workingDir, dataDir, debug)
	if err != nil {
		return nil, err
	}
	if err := ensureRuntimeDirectories(store.Config()); err != nil {
		return nil, err
	}
	return store, nil
}

func ensureRuntimeDirectories(cfg *Config) error {
	dirs := []string{
		filepath.Join(home.Dir(), "."+appName),
		filepath.Join(home.Dir(), "."+appName, "logs"),
		filepath.Join(home.Dir(), "."+appName, "skills-sh-cache"),
		filepath.Dir(GlobalConfig()),
		filepath.Dir(GlobalConfigData()),
	}

	if cfg != nil && cfg.Options != nil && strings.TrimSpace(cfg.Options.DataDirectory) != "" {
		dataDir := strings.TrimSpace(cfg.Options.DataDirectory)
		dirs = append(dirs,
			dataDir,
			filepath.Join(dataDir, "logs"),
			filepath.Join(dataDir, "skills-sh-cache"),
		)
	}

	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to initialize runtime directory %q: %w", dir, err)
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(dir, 0o700); err != nil {
				return fmt.Errorf("failed to set permissions on runtime directory %q: %w", dir, err)
			}
		}
	}

	return nil
}

func ProjectNeedsInitialization(store *ConfigStore) (bool, error) {
	if store == nil {
		return false, fmt.Errorf("config not loaded")
	}

	cfg := store.Config()
	flagFilePath := filepath.Join(cfg.Options.DataDirectory, InitFlagFilename)

	_, err := os.Stat(flagFilePath)
	if err == nil {
		return false, nil
	}

	if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check init flag file: %w", err)
	}

	someContextFileExists, err := contextPathsExist(store.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check for context files: %w", err)
	}
	if someContextFileExists {
		return false, nil
	}

	// If the working directory has no non-ignored files, skip initialization step
	empty, err := dirHasNoVisibleFiles(store.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check if directory is empty: %w", err)
	}
	if empty {
		return false, nil
	}

	return true, nil
}

func contextPathsExist(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	// Create a slice of lowercase filenames for lookup with slices.Contains
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, strings.ToLower(entry.Name()))
		}
	}

	// Check if any of the default context paths exist in the directory
	for _, path := range defaultContextPaths {
		// Extract just the filename from the path
		_, filename := filepath.Split(path)
		filename = strings.ToLower(filename)

		if slices.Contains(files, filename) {
			return true, nil
		}
	}

	return false, nil
}

// dirHasNoVisibleFiles returns true if the directory has no files/dirs after applying ignore rules.
func dirHasNoVisibleFiles(dir string) (bool, error) {
	files, _, err := fsext.ListDirectory(dir, nil, 1, 1)
	if err != nil {
		return false, err
	}
	return len(files) == 0, nil
}

func MarkProjectInitialized(store *ConfigStore) error {
	if store == nil {
		return fmt.Errorf("config not loaded")
	}
	flagFilePath := filepath.Join(store.Config().Options.DataDirectory, InitFlagFilename)

	file, err := os.Create(flagFilePath)
	if err != nil {
		return fmt.Errorf("failed to create init flag file: %w", err)
	}
	defer file.Close()

	return nil
}

func HasInitialDataConfig(store *ConfigStore) bool {
	if store == nil {
		return false
	}
	cfgPath := GlobalConfigData()
	if _, err := os.Stat(cfgPath); err != nil {
		return false
	}
	return store.Config().IsConfigured()
}

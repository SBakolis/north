package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths contains every mutable root used by North and the OpenCode integration.
type Paths struct {
	ConfigDir   string
	DataDir     string
	StateDir    string
	CacheDir    string
	OpenCodeDir string
}

type Environment interface {
	Getenv(string) string
	UserHomeDir() (string, error)
}

type OSEnvironment struct{}

func (OSEnvironment) Getenv(key string) string     { return os.Getenv(key) }
func (OSEnvironment) UserHomeDir() (string, error) { return os.UserHomeDir() }

// ResolvePaths honors XDG roots while making path resolution injectable in tests.
func ResolvePaths(env Environment) (Paths, error) {
	home, err := env.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" || !filepath.IsAbs(home) {
		return Paths{}, fmt.Errorf("resolve home directory: expected absolute path")
	}

	configRoot := rootOrDefault(env.Getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config"))
	dataRoot := rootOrDefault(env.Getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share"))
	stateRoot := rootOrDefault(env.Getenv("XDG_STATE_HOME"), filepath.Join(home, ".local", "state"))
	cacheRoot := rootOrDefault(env.Getenv("XDG_CACHE_HOME"), filepath.Join(home, ".cache"))

	for name, root := range map[string]string{
		"config": configRoot, "data": dataRoot, "state": stateRoot, "cache": cacheRoot,
	} {
		if !filepath.IsAbs(root) {
			return Paths{}, fmt.Errorf("resolve %s directory: expected absolute path", name)
		}
	}

	return Paths{
		ConfigDir:   filepath.Join(configRoot, "north"),
		DataDir:     filepath.Join(dataRoot, "north"),
		StateDir:    filepath.Join(stateRoot, "north"),
		CacheDir:    filepath.Join(cacheRoot, "north"),
		OpenCodeDir: filepath.Join(configRoot, "opencode"),
	}, nil
}

func rootOrDefault(value, fallback string) string {
	if value != "" {
		return filepath.Clean(value)
	}
	return fallback
}

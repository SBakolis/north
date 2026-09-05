package platform

import (
	"errors"
	"path/filepath"
	"testing"
)

type fakeEnvironment struct {
	home string
	err  error
	env  map[string]string
}

func (f fakeEnvironment) Getenv(key string) string     { return f.env[key] }
func (f fakeEnvironment) UserHomeDir() (string, error) { return f.home, f.err }

func TestResolvePathsDefaults(t *testing.T) {
	paths, err := ResolvePaths(fakeEnvironment{home: "/home/north", env: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]string{
		"config":   filepath.Join("/home/north", ".config", "north"),
		"data":     filepath.Join("/home/north", ".local", "share", "north"),
		"state":    filepath.Join("/home/north", ".local", "state", "north"),
		"cache":    filepath.Join("/home/north", ".cache", "north"),
		"opencode": filepath.Join("/home/north", ".config", "opencode"),
	}
	gots := map[string]string{"config": paths.ConfigDir, "data": paths.DataDir, "state": paths.StateDir, "cache": paths.CacheDir, "opencode": paths.OpenCodeDir}
	for key, want := range wants {
		if gots[key] != want {
			t.Errorf("%s = %q, want %q", key, gots[key], want)
		}
	}
}

func TestResolvePathsXDGOverrides(t *testing.T) {
	env := fakeEnvironment{home: "/home/north", env: map[string]string{
		"XDG_CONFIG_HOME": "/xdg/config", "XDG_DATA_HOME": "/xdg/data",
		"XDG_STATE_HOME": "/xdg/state", "XDG_CACHE_HOME": "/xdg/cache",
	}}
	paths, err := ResolvePaths(env)
	if err != nil {
		t.Fatal(err)
	}
	if paths.OpenCodeDir != "/xdg/config/opencode" || paths.StateDir != "/xdg/state/north" {
		t.Fatalf("unexpected paths: %+v", paths)
	}
}

func TestResolvePathsRejectsInvalidInputs(t *testing.T) {
	for _, env := range []fakeEnvironment{
		{err: errors.New("no home")},
		{home: "relative", env: map[string]string{}},
		{home: "/home/north", env: map[string]string{"XDG_CACHE_HOME": "relative"}},
	} {
		if _, err := ResolvePaths(env); err == nil {
			t.Fatal("expected path resolution error")
		}
	}
}

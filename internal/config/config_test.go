package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvStateDir, "")
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvAPISecret, "")
	t.Setenv(EnvStationID, "")
	t.Setenv(EnvInverterID, "")
	t.Setenv(EnvTheme, "")
	return home
}

func TestLoadCreatesDefaults(t *testing.T) {
	home := isolateHome(t)
	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", AppName, "config.toml"); paths.ConfigPath != want {
		t.Errorf("config path = %q, want %q", paths.ConfigPath, want)
	}
	if want := filepath.Join(home, ".local", "state", AppName); paths.StateDir != want {
		t.Errorf("state dir = %q, want %q", paths.StateDir, want)
	}

	res, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NoticeMentions("config") {
		t.Errorf("notice = %q", res.Notice)
	}
	info, err := os.Stat(paths.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config mode = %o, want 600", info.Mode().Perm())
	}
	if res.Config.Display.Currency != "PKR" {
		t.Errorf("currency = %q, want PKR", res.Config.Display.Currency)
	}
	if res.Config.PollIntervalDuration() != 5*time.Minute {
		t.Errorf("poll = %s", res.Config.PollIntervalDuration())
	}
}

// NoticeMentions keeps the test readable without depending on exact wording.
func (r LoadResult) NoticeMentions(sub string) bool {
	return len(r.Notice) > 0 && containsFold(r.Notice, sub)
}

func containsFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestLoadImportsLegacyConfig(t *testing.T) {
	home := isolateHome(t)
	legacyDir := filepath.Join(home, ".config", "solis2kde")
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := "api_key: \"legacy-key\"\napi_secret: \"legacy-secret\"\nstation_id: \"station-9\"\npoll_interval: 120\n"
	if err := os.WriteFile(filepath.Join(legacyDir, "config.yaml"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.Solis.APIKey != "legacy-key" || res.Config.Solis.StationID != "station-9" {
		t.Errorf("imported config = %+v", res.Config.Solis)
	}
	if res.Config.PollIntervalDuration() != 2*time.Minute {
		t.Errorf("poll = %s, want 2m", res.Config.PollIntervalDuration())
	}
	if !res.Imported {
		t.Error("Imported should be true")
	}
}

func TestEnvOverrides(t *testing.T) {
	isolateHome(t)
	paths, err := ResolvePaths("", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(paths); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvAPIKey, "env-key")
	t.Setenv(EnvAPISecret, "env-secret")
	t.Setenv(EnvStationID, "env-station")
	t.Setenv(EnvTheme, "transparent")

	res, err := Load(paths)
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.Solis.APIKey != "env-key" || res.Config.Solis.APISecret != "env-secret" || res.Config.Solis.StationID != "env-station" {
		t.Errorf("env overrides not applied: %+v", res.Config.Solis)
	}
	if res.Config.Display.Theme != "transparent" {
		t.Errorf("theme = %q", res.Config.Display.Theme)
	}
}

func TestValidate(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err == nil {
		t.Error("empty credentials should fail validation")
	}
	cfg.Solis.APIKey = "k"
	cfg.Solis.APISecret = "s"
	cfg.Solis.StationID = "1"
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
	cfg.Poll.Interval = "10s"
	if err := cfg.Validate(); err == nil {
		t.Error("poll below one minute should fail validation")
	}
}

func TestInitDoesNotOverwrite(t *testing.T) {
	home := isolateHome(t)
	path := filepath.Join(home, "config.toml")
	if err := Init(path); err != nil {
		t.Fatal(err)
	}
	if err := Init(path); err == nil {
		t.Error("Init must not overwrite an existing file")
	}
}

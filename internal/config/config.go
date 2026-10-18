// Package config loads and validates the TOML configuration, applies
// environment overrides, and imports credentials from the older KDE project.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	AppName       = "solar-monitor"
	DisplayName   = "solar-monitor-go"
	EnvConfig     = "monitor_CONFIG"
	EnvStateDir   = "monitor_STATE_DIR"
	EnvAPIKey     = "monitor_API_KEY"
	EnvAPISecret  = "monitor_API_SECRET"
	EnvStationID  = "monitor_STATION_ID"
	EnvInverterID = "monitor_INVERTER_ID"
	EnvTheme      = "monitor_THEME"
)

// LegacyConfigPath is the flat YAML file used by solis-kde-plasma-sensors.
const LegacyConfigPath = "~/.config/solis2kde/config.yaml"

// Config is the whole user configuration.
type Config struct {
	Solis   Solis   `toml:"solis"`
	Poll    Poll    `toml:"poll"`
	Display Display `toml:"display"`
	Tariff  Tariff  `toml:"tariff"`
	Storage Storage `toml:"storage"`
	Demo    Demo    `toml:"demo"`
}

type Solis struct {
	BaseURL         string `toml:"base_url"`
	APIKey          string `toml:"api_key"`
	APISecret       string `toml:"api_secret"`
	StationID       string `toml:"station_id"`
	InverterID      string `toml:"inverter_id"`
	InverterSN      string `toml:"inverter_sn"`
	Timeout         string `toml:"timeout"`
	GridPositive    string `toml:"grid_positive"`    // export or import
	BatteryPositive string `toml:"battery_positive"` // charge or discharge
}

type Poll struct {
	Interval string `toml:"interval"`
	Alarms   string `toml:"alarms"`
	History  string `toml:"history"`
}

type Display struct {
	Theme          string  `toml:"theme"`
	Motion         bool    `toml:"motion"`
	Units          string  `toml:"units"`
	Currency       string  `toml:"currency"`
	CurrencySymbol string  `toml:"currency_symbol"`
	CO2KgPerKWh    float64 `toml:"co2_kg_per_kwh"`
	// DayWindow is "day" for a fixed 00:00-23:59 chart axis, or "auto" for a
	// rolling window that fits the samples to the width.
	DayWindow string `toml:"day_window"`
	// ChartStyle is solid, area, box, quadrant, braille, ascii or graph.
	ChartStyle string `toml:"chart_style"`
}

type Tariff struct {
	Import float64 `toml:"import"`
	Export float64 `toml:"export"`
}

type Storage struct {
	Path            string `toml:"path"`
	RawSnapshotDays int    `toml:"raw_snapshot_days"`
}

type Demo struct {
	Enabled bool    `toml:"enabled"`
	Speed   float64 `toml:"speed"`
}

// Defaults returns the configuration used on first run.
func Defaults() Config {
	return Config{
		Solis: Solis{
			BaseURL:         "https://www.soliscloud.com:13333",
			Timeout:         "30s",
			GridPositive:    "export",
			BatteryPositive: "charge",
		},
		Poll: Poll{
			Interval: "5m",
			Alarms:   "15m",
			History:  "15m",
		},
		Display: Display{
			Theme:          "solar-dark",
			Motion:         true,
			Units:          "auto",
			Currency:       "PKR",
			CurrencySymbol: "₨",
			CO2KgPerKWh:    0.4,
			DayWindow:      "day",
			ChartStyle:     "graph",
		},
		Storage: Storage{
			RawSnapshotDays: 30,
		},
		Demo: Demo{
			Speed: 0.5,
		},
	}
}

// Paths are the resolved file locations for this run.
type Paths struct {
	ConfigPath string
	StateDir   string
	DBPath     string
}

// ResolvePaths determines config and state locations from the flag, env, and
// XDG defaults. An empty flag means "use the environment or default".
func ResolvePaths(configFlag, stateFlag string) (Paths, error) {
	cfgPath := configFlag
	if cfgPath == "" {
		cfgPath = os.Getenv(EnvConfig)
	}
	if cfgPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, fmt.Errorf("config: locate user config dir: %w", err)
		}
		cfgPath = filepath.Join(dir, AppName, "config.toml")
	}
	cfgPath = ExpandHome(cfgPath)

	stateDir := stateFlag
	if stateDir == "" {
		stateDir = os.Getenv(EnvStateDir)
	}
	if stateDir == "" {
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			stateDir = filepath.Join(xdg, AppName)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, fmt.Errorf("config: locate home dir: %w", err)
			}
			stateDir = filepath.Join(home, ".local", "state", AppName)
		}
	}
	stateDir = ExpandHome(stateDir)

	return Paths{
		ConfigPath: cfgPath,
		StateDir:   stateDir,
		DBPath:     filepath.Join(stateDir, "monitor.db"),
	}, nil
}

// LoadResult describes what happened while resolving the config.
type LoadResult struct {
	Config   Config
	Paths    Paths
	Existed  bool
	Imported bool
	Notice   string
}

// Load reads the config, creating it from defaults or the legacy KDE file when
// it does not exist yet. Environment overrides are applied last.
func Load(paths Paths) (LoadResult, error) {
	res := LoadResult{Paths: paths}

	if _, err := os.Stat(paths.ConfigPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return res, fmt.Errorf("config: stat %s: %w", paths.ConfigPath, err)
		}
		res.Config = Defaults()
		if importLegacy(&res.Config) {
			res.Imported = true
			res.Notice = "Imported SolisCloud credentials from " + ExpandHome(LegacyConfigPath)
		}
		if err := Save(paths.ConfigPath, res.Config); err != nil {
			return res, fmt.Errorf("config: write initial %s: %w", paths.ConfigPath, err)
		}
		if res.Notice == "" {
			res.Notice = "Created a default config at " + paths.ConfigPath
		}
	} else {
		b, err := os.ReadFile(paths.ConfigPath)
		if err != nil {
			return res, fmt.Errorf("config: read %s: %w", paths.ConfigPath, err)
		}
		if err := toml.Unmarshal(b, &res.Config); err != nil {
			return res, fmt.Errorf("config: parse %s: %w", paths.ConfigPath, err)
		}
		res.Existed = true
	}

	applyDefaults(&res.Config)
	applyEnv(&res.Config)
	return res, nil
}

// Init writes a commented template and does not overwrite an existing file.
func Init(path string) error {
	path = ExpandHome(path)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s", path)
	}
	if err := Save(path, Defaults()); err != nil {
		return err
	}
	return nil
}

// Save writes the config atomically with mode 0600.
func Save(path string, cfg Config) error {
	path = ExpandHome(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Validate checks the values needed for a real (non-demo) run.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Solis.APIKey) == "" {
		return errors.New("solis.api_key is empty")
	}
	if strings.TrimSpace(c.Solis.APISecret) == "" {
		return errors.New("solis.api_secret is empty")
	}
	if strings.TrimSpace(c.Solis.StationID) == "" {
		return errors.New("solis.station_id is empty")
	}
	if _, err := c.PollInterval(); err != nil {
		return err
	}
	if c.PollIntervalDuration() < time.Minute {
		return fmt.Errorf("poll.interval is %s; 1m is the minimum", c.PollIntervalDuration())
	}
	return nil
}

// PollInterval returns the configured detail poll interval.
func (c Config) PollInterval() (time.Duration, error) {
	return parseDuration("poll.interval", c.Poll.Interval, 5*time.Minute)
}

// PollIntervalDuration is the resolved interval, falling back to 5 minutes.
func (c Config) PollIntervalDuration() time.Duration {
	d, err := c.PollInterval()
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

// AlarmsInterval returns the configured alarm refresh interval.
func (c Config) AlarmsInterval() time.Duration {
	d, err := parseDuration("poll.alarms", c.Poll.Alarms, 15*time.Minute)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

// HistoryInterval returns the configured history refresh interval.
func (c Config) HistoryInterval() time.Duration {
	d, err := parseDuration("poll.history", c.Poll.History, 15*time.Minute)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

// Timeout returns the HTTP timeout.
func (c Config) Timeout() time.Duration {
	d, err := parseDuration("solis.timeout", c.Solis.Timeout, 30*time.Second)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func parseDuration(name, raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s is not a duration: %q", name, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return d, nil
}

func applyDefaults(c *Config) {
	d := Defaults()
	if c.Solis.BaseURL == "" {
		c.Solis.BaseURL = d.Solis.BaseURL
	}
	if c.Solis.Timeout == "" {
		c.Solis.Timeout = d.Solis.Timeout
	}
	if c.Solis.GridPositive == "" {
		c.Solis.GridPositive = d.Solis.GridPositive
	}
	if c.Solis.BatteryPositive == "" {
		c.Solis.BatteryPositive = d.Solis.BatteryPositive
	}
	if c.Poll.Interval == "" {
		c.Poll.Interval = d.Poll.Interval
	}
	if c.Poll.Alarms == "" {
		c.Poll.Alarms = d.Poll.Alarms
	}
	if c.Poll.History == "" {
		c.Poll.History = d.Poll.History
	}
	if c.Display.Theme == "" {
		c.Display.Theme = d.Display.Theme
	}
	if c.Display.Units == "" {
		c.Display.Units = d.Display.Units
	}
	if c.Display.Currency == "" {
		c.Display.Currency = d.Display.Currency
	}
	if c.Display.CurrencySymbol == "" {
		c.Display.CurrencySymbol = d.Display.CurrencySymbol
	}
	if c.Display.CO2KgPerKWh == 0 {
		c.Display.CO2KgPerKWh = d.Display.CO2KgPerKWh
	}
	if c.Display.DayWindow == "" {
		c.Display.DayWindow = d.Display.DayWindow
	}
	if c.Display.ChartStyle == "" {
		c.Display.ChartStyle = d.Display.ChartStyle
	}
	if c.Storage.RawSnapshotDays == 0 {
		c.Storage.RawSnapshotDays = d.Storage.RawSnapshotDays
	}
	if c.Demo.Speed == 0 {
		c.Demo.Speed = d.Demo.Speed
	}
}

func applyEnv(c *Config) {
	if v := os.Getenv(EnvAPIKey); v != "" {
		c.Solis.APIKey = v
	}
	if v := os.Getenv(EnvAPISecret); v != "" {
		c.Solis.APISecret = v
	}
	if v := os.Getenv(EnvStationID); v != "" {
		c.Solis.StationID = v
	}
	if v := os.Getenv(EnvInverterID); v != "" {
		c.Solis.InverterID = v
	}
	if v := os.Getenv(EnvTheme); v != "" {
		c.Display.Theme = v
	}
}

// importLegacy reads the scalar key:value subset used by the old project.
func importLegacy(c *Config) bool {
	path := ExpandHome(LegacyConfigPath)
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, ":") {
			continue
		}
		key, val, _ := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "api_key":
			c.Solis.APIKey = val
			found = true
		case "api_secret":
			c.Solis.APISecret = val
			found = true
		case "station_id":
			c.Solis.StationID = val
			found = true
		case "inverter_id":
			c.Solis.InverterID = val
			found = true
		case "poll_interval":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.Poll.Interval = (time.Duration(n) * time.Second).String()
			}
		}
	}
	return found
}

// ExpandHome expands a leading ~ in a path.
func ExpandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

// Command solar-moonitor is a read-only TUI for monitoring a Solis inverter
// through the SolisCloud API, with a local history and a demo mode.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/farazmazhar/solar-moonitor-go/internal/config"
	"github.com/farazmazhar/solar-moonitor-go/internal/monitor"
	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
	"github.com/farazmazhar/solar-moonitor-go/internal/provider/demo"
	"github.com/farazmazhar/solar-moonitor-go/internal/provider/solis"
	"github.com/farazmazhar/solar-moonitor-go/internal/store"
	"github.com/farazmazhar/solar-moonitor-go/internal/ui"
)

var version = "0.1.0-dev"

func main() {
	var (
		configFlag = flag.String("config", "", "path to config.toml (default $MOONITOR_CONFIG)")
		stateFlag  = flag.String("state-dir", "", "state directory (default $MOONITOR_STATE_DIR or ~/.local/state/solar-moonitor)")
		demoFlag   = flag.Bool("demo", false, "run with simulated data; no credentials or network")
		demoSpeed  = flag.Float64("demo-speed", 0, "simulated minutes per real second in demo mode")
		initFlag   = flag.Bool("init", false, "write a config template and exit")
		discover   = flag.Bool("discover", false, "fetch and print the raw inverterDetail payload, then exit")
		dump       = flag.Bool("dump", false, "render one frame with demo data to stdout and exit")
		dumpTab    = flag.Int("dump-tab", 0, "tab index to render with --dump (0-6)")
		dumpSize   = flag.String("dump-size", "80x24", "terminal size for --dump, e.g. 100x30")
		themeFlag  = flag.String("theme", "", "override the theme (solar-dark or transparent)")
		noMotion   = flag.Bool("no-motion", false, "disable animation")
		showVer    = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("solar-moonitor-go %s\n", version)
		return
	}

	paths, err := config.ResolvePaths(*configFlag, *stateFlag)
	if err != nil {
		fatal(err)
	}

	if *initFlag {
		if err := config.Init(paths.ConfigPath); err != nil {
			fatal(err)
		}
		fmt.Printf("wrote config template to %s\n", paths.ConfigPath)
		return
	}

	res, err := config.Load(paths)
	if err != nil {
		fatal(err)
	}
	cfg := res.Config
	if *themeFlag != "" {
		cfg.Display.Theme = *themeFlag
	}
	if *noMotion {
		cfg.Display.Motion = false
	}
	if *demoFlag {
		cfg.Demo.Enabled = true
	}
	if *demoSpeed > 0 {
		cfg.Demo.Speed = *demoSpeed
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *discover {
		if err := runDiscover(ctx, cfg); err != nil {
			fatal(err)
		}
		return
	}

	src, isDemo, setupErr := buildSource(cfg)

	var rediscover func(context.Context) error
	if client, ok := src.(*solis.Client); ok {
		rediscover = func(ctx context.Context) error {
			client.ResetInverter()
			_, err := client.Snapshot(ctx)
			return err
		}
	}

	var localStore *store.Store
	storeNotice := ""
	if src != nil && !isDemo {
		opened, err := store.Open(paths.DBPath)
		if err != nil {
			storeNotice = "local history unavailable: " + err.Error()
		} else {
			localStore = opened
		}
	}
	var service *monitor.Service
	if localStore != nil {
		service = monitor.New(src, localStore, cfg.Storage.RawSnapshotDays)
		src = service
	}

	if *dump {
		if src == nil {
			src = demo.New(demo.Options{Speed: cfg.Demo.Speed})
			isDemo = true
		}
		w, h := parseSize(*dumpSize)
		frame, err := ui.DumpFrame(src, cfg, paths, version, isDemo, ui.Tab(clampInt(*dumpTab, 0, 6)), w, h)
		if err != nil {
			fatal(err)
		}
		fmt.Println(frame)
		return
	}

	var (
		seedSnap    *provider.Snapshot
		seedSamples []provider.Sample
	)
	if service != nil {
		if snap, samples, ok := service.Seed(ctx); ok {
			seedSnap = &snap
			seedSamples = samples
		}
	}

	var statsFn func() (ui.StoreStats, bool)
	if localStore != nil {
		statsFn = func() (ui.StoreStats, bool) {
			sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			st, err := localStore.Stats(sctx)
			if err != nil {
				return ui.StoreStats{}, false
			}
			return ui.StoreStats{
				Path: st.Path, SizeBytes: st.SizeBytes, Samples: st.Samples,
				Oldest: st.Oldest, Newest: st.Newest,
			}, true
		}
	}

	model := ui.New(ui.Options{
		Config:       cfg,
		Paths:        paths,
		Source:       src,
		Demo:         isDemo,
		Version:      version,
		SetupErr:     setupErr,
		SeedSnapshot: seedSnap,
		SeedSamples:  seedSamples,
		Stats:        statsFn,
		Rediscover:   rediscover,
		DemoFactory: func() provider.Source {
			return demo.New(demo.Options{Speed: cfg.Demo.Speed})
		},
	})

	program := tea.NewProgram(model, tea.WithContext(ctx))
	if res.Notice != "" {
		model.SetNotice(res.Notice)
	} else if storeNotice != "" {
		model.SetNotice(storeNotice)
	}
	if _, err := program.Run(); err != nil && !errors.Is(err, context.Canceled) {
		fatal(err)
	}
}

func buildSource(cfg config.Config) (provider.Source, bool, error) {
	if cfg.Demo.Enabled {
		return demo.New(demo.Options{Speed: cfg.Demo.Speed}), true, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, false, err
	}
	client, err := solis.New(solis.Options{
		BaseURL:         cfg.Solis.BaseURL,
		APIKey:          cfg.Solis.APIKey,
		APISecret:       cfg.Solis.APISecret,
		StationID:       cfg.Solis.StationID,
		InverterID:      cfg.Solis.InverterID,
		InverterSN:      cfg.Solis.InverterSN,
		Timeout:         cfg.Timeout(),
		Currency:        cfg.Display.Currency,
		GridPositive:    cfg.Solis.GridPositive,
		BatteryPositive: cfg.Solis.BatteryPositive,
		Location:        time.Local,
	})
	if err != nil {
		return nil, false, err
	}
	return client, false, nil
}

func runDiscover(ctx context.Context, cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("discover needs credentials: %w", err)
	}
	client, err := solis.New(solis.Options{
		BaseURL:         cfg.Solis.BaseURL,
		APIKey:          cfg.Solis.APIKey,
		APISecret:       cfg.Solis.APISecret,
		StationID:       cfg.Solis.StationID,
		InverterID:      cfg.Solis.InverterID,
		InverterSN:      cfg.Solis.InverterSN,
		Timeout:         cfg.Timeout(),
		Currency:        cfg.Display.Currency,
		GridPositive:    cfg.Solis.GridPositive,
		BatteryPositive: cfg.Solis.BatteryPositive,
		Location:        time.Local,
	})
	if err != nil {
		return err
	}
	raw, err := client.RawDetail(ctx)
	if err != nil {
		return err
	}
	var pretty []byte
	if pretty, err = json.MarshalIndent(json.RawMessage(raw), "", "  "); err != nil {
		pretty = raw
	}
	fmt.Println(string(pretty))

	fmt.Println()
	fmt.Println("normalised summary:")
	snap, err := solis.NormalizeSnapshot(raw, solis.NormalizeOptions{
		Now:             time.Now(),
		Location:        time.Local,
		Currency:        cfg.Display.Currency,
		GridPositive:    cfg.Solis.GridPositive,
		BatteryPositive: cfg.Solis.BatteryPositive,
		Source:          "solis",
	})
	if err != nil {
		return err
	}
	snap.Raw = nil
	out, _ := json.MarshalIndent(snap, "", "  ")
	fmt.Println(string(out))
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "solar-moonitor:", err)
	os.Exit(1)
}

func parseSize(s string) (int, int) {
	parts := strings.SplitN(strings.ToLower(s), "x", 2)
	if len(parts) != 2 {
		return 80, 24
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w < 20 || h < 10 {
		return 80, 24
	}
	return w, h
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

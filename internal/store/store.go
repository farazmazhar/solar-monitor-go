// Package store keeps local history in a SQLite database: normalised
// snapshots, five-minute samples, daily and monthly aggregates, and alarms.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

// Store is a local history database.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens or creates the database at path. Use ":memory:" for a throwaway
// store, which is what demo mode should do.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("store: empty database path")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("store: create state dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// SQLite writes are serialised anyway; one connection avoids lock churn
	// between the poller and the UI.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: busy_timeout: %w", err)
	}
	if path != ":memory:" {
		_, _ = db.Exec("PRAGMA journal_mode = WAL")
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Path returns the database path.
func (s *Store) Path() string { return s.path }

const schema = `
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS snapshots (
	ts         INTEGER PRIMARY KEY,
	fetched_at INTEGER NOT NULL,
	data_ts    INTEGER NOT NULL,
	source     TEXT NOT NULL,
	payload    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS samples (
	ts        INTEGER PRIMARY KEY,
	solar_w   REAL,
	load_w    REAL,
	grid_w    REAL,
	battery_w REAL,
	soc       REAL,
	import_w  REAL,
	export_w  REAL
);
CREATE TABLE IF NOT EXISTS daily (
	date          TEXT PRIMARY KEY,
	generated_kwh REAL,
	consumed_kwh  REAL,
	imported_kwh  REAL,
	exported_kwh  REAL,
	charged_kwh   REAL,
	discharged_kwh REAL,
	income        REAL,
	currency      TEXT
);
CREATE TABLE IF NOT EXISTS monthly (
	month         TEXT PRIMARY KEY,
	generated_kwh REAL,
	consumed_kwh  REAL,
	imported_kwh  REAL,
	exported_kwh  REAL,
	charged_kwh   REAL,
	discharged_kwh REAL,
	income        REAL,
	currency      TEXT
);
CREATE TABLE IF NOT EXISTS alarms (
	id       TEXT PRIMARY KEY,
	level    INTEGER,
	code     TEXT,
	message  TEXT,
	advice   TEXT,
	state    TEXT,
	begin_ts INTEGER,
	end_ts   INTEGER,
	active   INTEGER
);
`

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
		return fmt.Errorf("store: set user_version: %w", err)
	}
	return nil
}

// SaveSnapshot records one snapshot and its five-minute sample. Repeated
// samples at the same instant replace the previous row.
func (s *Store) SaveSnapshot(ctx context.Context, snap provider.Snapshot) error {
	if s == nil {
		return nil
	}
	dataAt := snap.DataAt
	if dataAt.IsZero() {
		dataAt = snap.FetchedAt
	}
	if dataAt.IsZero() {
		dataAt = time.Now()
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("store: encode snapshot: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	ts := dataAt.UnixMilli()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO snapshots (ts, fetched_at, data_ts, source, payload) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(ts) DO UPDATE SET fetched_at=excluded.fetched_at, data_ts=excluded.data_ts,
		   source=excluded.source, payload=excluded.payload`,
		ts, snap.FetchedAt.UnixMilli(), dataAt.UnixMilli(), snap.Source, string(payload),
	); err != nil {
		return fmt.Errorf("store: insert snapshot: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO samples (ts, solar_w, load_w, grid_w, battery_w, soc, import_w, export_w)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(ts) DO UPDATE SET solar_w=excluded.solar_w, load_w=excluded.load_w,
		   grid_w=excluded.grid_w, battery_w=excluded.battery_w, soc=excluded.soc,
		   import_w=excluded.import_w, export_w=excluded.export_w`,
		ts,
		nullable(snap.SolarW), nullable(snap.LoadW), nullable(snap.GridW), nullable(snap.BatteryW),
		nullable(snap.SOC), nullable(snap.GridImportW), nullable(snap.GridExportW),
	); err != nil {
		return fmt.Errorf("store: insert sample: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// LatestSnapshot returns the most recent stored snapshot.
func (s *Store) LatestSnapshot(ctx context.Context) (provider.Snapshot, bool, error) {
	if s == nil {
		return provider.Snapshot{}, false, nil
	}
	var payload string
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM snapshots ORDER BY ts DESC LIMIT 1`).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return provider.Snapshot{}, false, nil
	}
	if err != nil {
		return provider.Snapshot{}, false, fmt.Errorf("store: latest snapshot: %w", err)
	}
	var snap provider.Snapshot
	if err := json.Unmarshal([]byte(payload), &snap); err != nil {
		return provider.Snapshot{}, false, fmt.Errorf("store: decode snapshot: %w", err)
	}
	return snap, true, nil
}

// Samples returns stored five-minute samples in [from, to).
func (s *Store) Samples(ctx context.Context, from, to time.Time) ([]provider.Sample, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT ts, solar_w, load_w, grid_w, battery_w, soc, import_w, export_w
		 FROM samples WHERE ts >= ? AND ts < ? ORDER BY ts`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("store: query samples: %w", err)
	}
	defer rows.Close()

	var out []provider.Sample
	for rows.Next() {
		var (
			ts                                        int64
			solar, load, grid, battery, soc, imp, exp sql.NullFloat64
		)
		if err := rows.Scan(&ts, &solar, &load, &grid, &battery, &soc, &imp, &exp); err != nil {
			return nil, fmt.Errorf("store: scan sample: %w", err)
		}
		out = append(out, provider.Sample{
			TS:       time.UnixMilli(ts),
			SolarW:   ptr(solar),
			LoadW:    ptr(load),
			GridW:    ptr(grid),
			BatteryW: ptr(battery),
			SOC:      ptr(soc),
			ImportW:  ptr(imp),
			ExportW:  ptr(exp),
		})
	}
	return out, rows.Err()
}

// SaveDayEnergy upserts one month of daily rows.
func (s *Store) SaveDayEnergy(ctx context.Context, rows []provider.DayEnergy) error {
	if s == nil || len(rows) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO daily (date, generated_kwh, consumed_kwh, imported_kwh, exported_kwh, charged_kwh, discharged_kwh, income, currency)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(date) DO UPDATE SET generated_kwh=excluded.generated_kwh, consumed_kwh=excluded.consumed_kwh,
			   imported_kwh=excluded.imported_kwh, exported_kwh=excluded.exported_kwh, charged_kwh=excluded.charged_kwh,
			   discharged_kwh=excluded.discharged_kwh, income=excluded.income, currency=excluded.currency`,
			r.Date.Format("2006-01-02"),
			nullable(r.GeneratedKWh), nullable(r.ConsumedKWh), nullable(r.ImportedKWh), nullable(r.ExportedKWh),
			nullable(r.ChargedKWh), nullable(r.DischargedKWh), nullable(r.Income), r.Currency,
		); err != nil {
			return fmt.Errorf("store: upsert daily: %w", err)
		}
	}
	return tx.Commit()
}

// DayEnergy returns stored daily rows for a month.
func (s *Store) DayEnergy(ctx context.Context, year int, month time.Month) ([]provider.DayEnergy, error) {
	if s == nil {
		return nil, nil
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	next := first.AddDate(0, 1, 0)
	rows, err := s.db.QueryContext(ctx,
		`SELECT date, generated_kwh, consumed_kwh, imported_kwh, exported_kwh, charged_kwh, discharged_kwh, income, currency
		 FROM daily WHERE date >= ? AND date < ? ORDER BY date`,
		first.Format("2006-01-02"), next.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("store: query daily: %w", err)
	}
	defer rows.Close()

	var out []provider.DayEnergy
	for rows.Next() {
		var (
			date                string
			gen, cons, imp, exp sql.NullFloat64
			chg, dis, income    sql.NullFloat64
			currency            sql.NullString
		)
		if err := rows.Scan(&date, &gen, &cons, &imp, &exp, &chg, &dis, &income, &currency); err != nil {
			return nil, fmt.Errorf("store: scan daily: %w", err)
		}
		t, err := time.ParseInLocation("2006-01-02", date, time.Local)
		if err != nil {
			continue
		}
		out = append(out, provider.DayEnergy{
			Date: t, GeneratedKWh: ptr(gen), ConsumedKWh: ptr(cons), ImportedKWh: ptr(imp),
			ExportedKWh: ptr(exp), ChargedKWh: ptr(chg), DischargedKWh: ptr(dis),
			Income: ptr(income), Currency: currency.String,
		})
	}
	return out, rows.Err()
}

// SaveMonthEnergy upserts one year of monthly rows.
func (s *Store) SaveMonthEnergy(ctx context.Context, rows []provider.MonthEnergy) error {
	if s == nil || len(rows) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO monthly (month, generated_kwh, consumed_kwh, imported_kwh, exported_kwh, charged_kwh, discharged_kwh, income, currency)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(month) DO UPDATE SET generated_kwh=excluded.generated_kwh, consumed_kwh=excluded.consumed_kwh,
			   imported_kwh=excluded.imported_kwh, exported_kwh=excluded.exported_kwh, charged_kwh=excluded.charged_kwh,
			   discharged_kwh=excluded.discharged_kwh, income=excluded.income, currency=excluded.currency`,
			r.Month.Format("2006-01"),
			nullable(r.GeneratedKWh), nullable(r.ConsumedKWh), nullable(r.ImportedKWh), nullable(r.ExportedKWh),
			nullable(r.ChargedKWh), nullable(r.DischargedKWh), nullable(r.Income), r.Currency,
		); err != nil {
			return fmt.Errorf("store: upsert monthly: %w", err)
		}
	}
	return tx.Commit()
}

// MonthEnergy returns stored monthly rows for a year.
func (s *Store) MonthEnergy(ctx context.Context, year int) ([]provider.MonthEnergy, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT month, generated_kwh, consumed_kwh, imported_kwh, exported_kwh, charged_kwh, discharged_kwh, income, currency
		 FROM monthly WHERE month >= ? AND month < ? ORDER BY month`,
		fmt.Sprintf("%04d-01", year), fmt.Sprintf("%04d-01", year+1))
	if err != nil {
		return nil, fmt.Errorf("store: query monthly: %w", err)
	}
	defer rows.Close()

	var out []provider.MonthEnergy
	for rows.Next() {
		var (
			month               string
			gen, cons, imp, exp sql.NullFloat64
			chg, dis, income    sql.NullFloat64
			currency            sql.NullString
		)
		if err := rows.Scan(&month, &gen, &cons, &imp, &exp, &chg, &dis, &income, &currency); err != nil {
			return nil, fmt.Errorf("store: scan monthly: %w", err)
		}
		t, err := time.ParseInLocation("2006-01", month, time.Local)
		if err != nil {
			continue
		}
		out = append(out, provider.MonthEnergy{
			Month: t, GeneratedKWh: ptr(gen), ConsumedKWh: ptr(cons), ImportedKWh: ptr(imp),
			ExportedKWh: ptr(exp), ChargedKWh: ptr(chg), DischargedKWh: ptr(dis),
			Income: ptr(income), Currency: currency.String,
		})
	}
	return out, rows.Err()
}

// SaveAlarms upserts alarms.
func (s *Store) SaveAlarms(ctx context.Context, alarms []provider.Alarm) error {
	if s == nil || len(alarms) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, a := range alarms {
		id := a.ID
		if id == "" {
			id = fmt.Sprintf("%s-%d", a.Code, a.Begin.Unix())
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO alarms (id, level, code, message, advice, state, begin_ts, end_ts, active)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET level=excluded.level, code=excluded.code, message=excluded.message,
			   advice=excluded.advice, state=excluded.state, begin_ts=excluded.begin_ts, end_ts=excluded.end_ts,
			   active=excluded.active`,
			id, a.Level, a.Code, a.Message, a.Advice, a.State,
			a.Begin.UnixMilli(), a.End.UnixMilli(), boolInt(a.Active),
		); err != nil {
			return fmt.Errorf("store: upsert alarm: %w", err)
		}
	}
	return tx.Commit()
}

// Alarms returns stored alarms in [from, to].
func (s *Store) Alarms(ctx context.Context, from, to time.Time) ([]provider.Alarm, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, level, code, message, advice, state, begin_ts, end_ts, active
		 FROM alarms WHERE begin_ts >= ? AND begin_ts <= ? ORDER BY begin_ts DESC`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("store: query alarms: %w", err)
	}
	defer rows.Close()

	var out []provider.Alarm
	for rows.Next() {
		var (
			a             provider.Alarm
			begin, end    int64
			active        int
			advice, state sql.NullString
		)
		if err := rows.Scan(&a.ID, &a.Level, &a.Code, &a.Message, &advice, &state, &begin, &end, &active); err != nil {
			return nil, fmt.Errorf("store: scan alarm: %w", err)
		}
		a.Advice = advice.String
		a.State = state.String
		a.Begin = time.UnixMilli(begin)
		if end > 0 {
			a.End = time.UnixMilli(end)
		}
		a.Active = active != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

// PruneSnapshots deletes raw snapshot payloads older than days. Samples and
// aggregates are kept forever.
func (s *Store) PruneSnapshots(ctx context.Context, days int) error {
	if s == nil || days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days).UnixMilli()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE ts < ?`, cutoff); err != nil {
		return fmt.Errorf("store: prune snapshots: %w", err)
	}
	return nil
}

// Stats summarises the database for the diagnostics screen.
type Stats struct {
	Path      string
	SizeBytes int64
	Samples   int
	Oldest    time.Time
	Newest    time.Time
}

// Stats reports row counts and file size.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	if s == nil {
		return Stats{}, nil
	}
	st := Stats{Path: s.path}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples`).Scan(&st.Samples); err != nil {
		return st, err
	}
	var oldest, newest sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(ts), MAX(ts) FROM samples`).Scan(&oldest, &newest); err != nil {
		return st, err
	}
	if oldest.Valid {
		st.Oldest = time.UnixMilli(oldest.Int64)
	}
	if newest.Valid {
		st.Newest = time.UnixMilli(newest.Int64)
	}
	if s.path != ":memory:" {
		if info, err := os.Stat(s.path); err == nil {
			st.SizeBytes = info.Size()
		}
	}
	return st, nil
}

func nullable(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return provider.Float(v.Float64)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

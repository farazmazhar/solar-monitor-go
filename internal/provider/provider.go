// Package provider defines the provider-neutral domain model shared by the
// SolisCloud client, the demo simulator, the store and the UI.
package provider

import (
	"context"
	"encoding/json"
	"time"
)

// Source is one solar data provider. The UI and poller depend on this
// interface only; provider-specific code stays behind it.
type Source interface {
	Name() string
	Snapshot(ctx context.Context) (Snapshot, error)
	Today(ctx context.Context, day time.Time) ([]Sample, error)
	Month(ctx context.Context, year int, month time.Month) ([]DayEnergy, error)
	Year(ctx context.Context, year int) ([]MonthEnergy, error)
	Alarms(ctx context.Context, from, to time.Time) ([]Alarm, error)
	Close() error
}

// YearDaysSource is an optional provider capability: daily rows for a whole
// year. It backs the calendar heatmap without the UI fetching twelve months
// itself.
type YearDaysSource interface {
	YearDays(ctx context.Context, year int) ([]DayEnergy, error)
}

// State is the inverter's own reported state.
type State string

const (
	StateOnline  State = "online"
	StateOffline State = "offline"
	StateAlarm   State = "alarm"
	StateUnknown State = "unknown"
)

// Snapshot is one normalised inverter reading. Powers are watts and energies
// are kilowatt-hours; missing readings stay nil rather than becoming zero.
//
// GridW is signed: positive means export, negative means import.
// BatteryW is signed: positive means charging, negative means discharging.
type Snapshot struct {
	FetchedAt time.Time `json:"fetched_at"`
	DataAt    time.Time `json:"data_at"`
	Source    string    `json:"source"`

	Station string `json:"station,omitempty"`
	Model   string `json:"model,omitempty"`
	SN      string `json:"sn,omitempty"`
	State   State  `json:"state,omitempty"`

	SolarW      *float64 `json:"solar_w,omitempty"`
	LoadW       *float64 `json:"load_w,omitempty"`
	BackupW     *float64 `json:"backup_w,omitempty"`
	GridW       *float64 `json:"grid_w,omitempty"`
	BatteryW    *float64 `json:"battery_w,omitempty"`
	GridImportW *float64 `json:"grid_import_w,omitempty"`
	GridExportW *float64 `json:"grid_export_w,omitempty"`

	SOC        *float64 `json:"soc,omitempty"`
	SOH        *float64 `json:"soh,omitempty"`
	TempC      *float64 `json:"temp_c,omitempty"`
	BatteryV   *float64 `json:"battery_v,omitempty"`
	BatteryA   *float64 `json:"battery_a,omitempty"`
	GridHz     *float64 `json:"grid_hz,omitempty"`
	GridV      *float64 `json:"grid_v,omitempty"`
	ReserveSOC *float64 `json:"reserve_soc,omitempty"`
	ChargeSOC  *float64 `json:"charge_soc,omitempty"`

	PVStrings []PVString `json:"pv_strings,omitempty"`

	TodayKWh          *float64 `json:"today_kwh,omitempty"`
	MonthKWh          *float64 `json:"month_kwh,omitempty"`
	YearKWh           *float64 `json:"year_kwh,omitempty"`
	TotalKWh          *float64 `json:"total_kwh,omitempty"`
	LoadTodayKWh      *float64 `json:"load_today_kwh,omitempty"`
	ImportTodayKWh    *float64 `json:"import_today_kwh,omitempty"`
	ExportTodayKWh    *float64 `json:"export_today_kwh,omitempty"`
	ChargeTodayKWh    *float64 `json:"charge_today_kwh,omitempty"`
	DischargeTodayKWh *float64 `json:"discharge_today_kwh,omitempty"`
	ChargeTotalKWh    *float64 `json:"charge_total_kwh,omitempty"`
	DischargeTotalKWh *float64 `json:"discharge_total_kwh,omitempty"`

	IncomeDay   *float64 `json:"income_day,omitempty"`
	IncomeMonth *float64 `json:"income_month,omitempty"`
	IncomeYear  *float64 `json:"income_year,omitempty"`
	IncomeTotal *float64 `json:"income_total,omitempty"`
	Currency    string   `json:"currency,omitempty"`

	Flow Flow `json:"flow"`

	Faults []string `json:"faults,omitempty"`

	// Raw is the provider payload this snapshot was built from. It is kept for
	// the diagnostics browser and is never used to render the dashboard.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// PVString is one DC input or MPPT channel.
type PVString struct {
	Index   int      `json:"index"`
	Voltage *float64 `json:"voltage,omitempty"`
	Current *float64 `json:"current,omitempty"`
	Power   *float64 `json:"power,omitempty"`
}

// Flow is a derived power-flow split in watts. It is only used to draw arrows;
// the authoritative values remain the signed readings on Snapshot.
type Flow struct {
	PVToHouse      float64 `json:"pv_to_house"`
	PVToBattery    float64 `json:"pv_to_battery"`
	PVToGrid       float64 `json:"pv_to_grid"`
	GridToHouse    float64 `json:"grid_to_house"`
	GridToBattery  float64 `json:"grid_to_battery"`
	BatteryToHouse float64 `json:"battery_to_house"`
	BatteryToGrid  float64 `json:"battery_to_grid"`
	Derived        bool    `json:"derived"`
	Balanced       bool    `json:"balanced"`
}

// Sample is one point on a day curve. It is the unit stored by the local
// history and returned by a provider's Today method.
type Sample struct {
	TS       time.Time `json:"ts"`
	SolarW   *float64  `json:"solar_w,omitempty"`
	LoadW    *float64  `json:"load_w,omitempty"`
	GridW    *float64  `json:"grid_w,omitempty"`
	BatteryW *float64  `json:"battery_w,omitempty"`
	SOC      *float64  `json:"soc,omitempty"`
	ImportW  *float64  `json:"import_w,omitempty"`
	ExportW  *float64  `json:"export_w,omitempty"`
}

// DayEnergy is one day of the month view.
type DayEnergy struct {
	Date          time.Time `json:"date"`
	GeneratedKWh  *float64  `json:"generated_kwh,omitempty"`
	ConsumedKWh   *float64  `json:"consumed_kwh,omitempty"`
	ImportedKWh   *float64  `json:"imported_kwh,omitempty"`
	ExportedKWh   *float64  `json:"exported_kwh,omitempty"`
	ChargedKWh    *float64  `json:"charged_kwh,omitempty"`
	DischargedKWh *float64  `json:"discharged_kwh,omitempty"`
	Income        *float64  `json:"income,omitempty"`
	Currency      string    `json:"currency,omitempty"`
}

// MonthEnergy is one month of the year view.
type MonthEnergy struct {
	Month         time.Time `json:"month"`
	GeneratedKWh  *float64  `json:"generated_kwh,omitempty"`
	ConsumedKWh   *float64  `json:"consumed_kwh,omitempty"`
	ImportedKWh   *float64  `json:"imported_kwh,omitempty"`
	ExportedKWh   *float64  `json:"exported_kwh,omitempty"`
	ChargedKWh    *float64  `json:"charged_kwh,omitempty"`
	DischargedKWh *float64  `json:"discharged_kwh,omitempty"`
	Income        *float64  `json:"income,omitempty"`
	Currency      string    `json:"currency,omitempty"`
}

// Alarm is one SolisCloud device alarm.
type Alarm struct {
	ID       string    `json:"id"`
	Level    int       `json:"level"`
	Code     string    `json:"code"`
	Message  string    `json:"message"`
	Advice   string    `json:"advice,omitempty"`
	State    string    `json:"state,omitempty"`
	DeviceSN string    `json:"device_sn,omitempty"`
	Begin    time.Time `json:"begin"`
	End      time.Time `json:"end,omitempty"`
	Active   bool      `json:"active"`
}

// Float returns a pointer to v. It is a convenience for building fixtures and
// normalised snapshots.
func Float(v float64) *float64 { return &v }

// Value returns the value behind p, or fallback when p is nil.
func Value(p *float64, fallback float64) float64 {
	if p == nil {
		return fallback
	}
	return *p
}

// Conv returns the value behind p, or fallback when p is nil.
func Conv(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

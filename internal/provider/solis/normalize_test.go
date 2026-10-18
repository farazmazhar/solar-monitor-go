package solis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

const detailFixture = `{
  "stationName": "Home Rooftop",
  "model": "S6-EH1P6K-L-PRO",
  "sn": "ABC123",
  "state": 1,
  "currentState": "3",
  "faultCodeDesc": "Generating",
  "timeStr": "2026-10-18 13:20:00",
  "pac": 5.5, "pacStr": "kW",
  "psumOrgin": 3200,
  "familyLoadPowerOrigin": 650,
  "backupPowerA": 250,
  "batteryPower": 1.8, "batteryPowerStr": "kW",
  "batteryCapacitySoc": 62,
  "batteryHealthSoh": 97.4,
  "storageBatteryVoltage": 49.8,
  "storageBatteryCurrent": 36.1,
  "inverterTemperature": 41.1,
  "fac": 50.01,
  "uAc1": 231.4,
  "eToday": 25.9, "eTodayStr": "kWh",
  "eMonth": 468, "eMonthStr": "kWh",
  "eTotal": 8.8, "eTotalStr": "MWh",
  "batteryTodayChargeEnergy": 9.9,
  "batteryTotalChargeEnergy": 2481.9,
  "gridPurchasedTodayEnergy": 3.5,
  "gridSellTodayEnergy": 11.7,
  "socDischargeSet": 20,
  "socChargingSet": 98,
  "dcInputType": 1,
  "uPv1": 356, "iPv1": 9.9, "pow1": 3524,
  "uPv2": 349, "iPv2": 6.2, "pow2": 2163,
  "sscCurrentFlowMap": {"pvToBattery": 1.8, "batteryToLoad": 0, "batteryToGrid": 0, "gridToLoad": 0}
}`

func TestNormalizeSnapshot(t *testing.T) {
	loc := time.FixedZone("PKT", 5*3600)
	now := time.Date(2026, 10, 18, 13, 20, 0, 0, loc)
	snap, err := NormalizeSnapshot(json.RawMessage(detailFixture), NormalizeOptions{
		Now: now, Location: loc, Currency: "PKR", Source: "solis",
	})
	if err != nil {
		t.Fatal(err)
	}

	check := func(name string, got *float64, want float64) {
		t.Helper()
		if got == nil {
			t.Fatalf("%s is nil, want %v", name, want)
		}
		if diff := *got - want; diff < -0.01 || diff > 0.01 {
			t.Errorf("%s = %v, want %v", name, *got, want)
		}
	}
	check("SolarW", snap.SolarW, 5500)
	check("GridW", snap.GridW, 3200)
	check("GridExportW", snap.GridExportW, 3200)
	check("GridImportW", snap.GridImportW, 0)
	check("LoadW", snap.LoadW, 650)
	check("BackupW", snap.BackupW, 250)
	check("BatteryW", snap.BatteryW, 1800)
	check("SOC", snap.SOC, 62)
	check("SOH", snap.SOH, 97.4)
	check("BatteryV", snap.BatteryV, 49.8)
	check("BatteryA", snap.BatteryA, 36.1)
	check("TempC", snap.TempC, 41.1)
	check("TodayKWh", snap.TodayKWh, 25.9)
	check("MonthKWh", snap.MonthKWh, 468)
	check("TotalKWh", snap.TotalKWh, 8800) // eTotal in MWh
	check("ChargeTodayKWh", snap.ChargeTodayKWh, 9.9)
	check("ReserveSOC", snap.ReserveSOC, 20)
	check("ChargeSOC", snap.ChargeSOC, 98)

	if snap.State != provider.StateOnline {
		t.Errorf("state = %q, want online", snap.State)
	}
	if snap.Station != "Home Rooftop" || snap.SN != "ABC123" {
		t.Errorf("identity = %q/%q", snap.Station, snap.SN)
	}
	if len(snap.PVStrings) != 2 {
		t.Fatalf("pv strings = %d, want 2", len(snap.PVStrings))
	}
	check("PV1 power", snap.PVStrings[0].Power, 3524)
	check("PV2 power", snap.PVStrings[1].Power, 2163)
	if snap.Flow.PVToBattery != 1800 {
		t.Errorf("flow pvToBattery = %v, want 1800", snap.Flow.PVToBattery)
	}
}

func TestNormalizeSnapshotSignOverrides(t *testing.T) {
	snap, err := NormalizeSnapshot(json.RawMessage(detailFixture), NormalizeOptions{
		Now: time.Now(), Currency: "PKR",
		GridPositive: "import", BatteryPositive: "discharge",
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.GridW == nil || *snap.GridW != -3200 {
		t.Errorf("grid = %v, want -3200", snap.GridW)
	}
	if snap.GridImportW == nil || *snap.GridImportW != 3200 {
		t.Errorf("import = %v, want 3200", snap.GridImportW)
	}
	if snap.BatteryW == nil || *snap.BatteryW != -1800 {
		t.Errorf("battery = %v, want -1800", snap.BatteryW)
	}
}

func TestNormalizeSnapshotAliases(t *testing.T) {
	raw := json.RawMessage(`{
	  "familyLoadPower": 0.9, "familyLoadPowerStr": "kW",
	  "bypassLoadPower": 0.3, "bypassLoadPowerStr": "kW",
	  "psum": 1.2, "psumStr": "kW",
	  "batteryPercent": 44
	}`)
	snap, err := NormalizeSnapshot(raw, NormalizeOptions{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if snap.LoadW == nil || *snap.LoadW != 900 {
		t.Errorf("load = %v, want 900", snap.LoadW)
	}
	if snap.BackupW == nil || *snap.BackupW != 300 {
		t.Errorf("backup = %v, want 300", snap.BackupW)
	}
	if snap.GridW == nil || *snap.GridW != 1200 {
		t.Errorf("grid = %v, want 1200", snap.GridW)
	}
	if snap.SOC == nil || *snap.SOC != 44 {
		t.Errorf("soc = %v, want 44", snap.SOC)
	}
}

func TestNormalizeSnapshotMissingValuesStayNil(t *testing.T) {
	snap, err := NormalizeSnapshot(json.RawMessage(`{"sn":"X"}`), NormalizeOptions{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if snap.SolarW != nil || snap.GridW != nil || snap.SOC != nil {
		t.Fatal("missing readings must stay nil rather than becoming zero")
	}
}

func TestDeriveFlowBalance(t *testing.T) {
	f := deriveFlow(5500, 650, 3200, 1800)
	if !f.Balanced {
		t.Errorf("flow should balance: %+v", f)
	}
	if f.PVToHouse != 650 {
		t.Errorf("pvToHouse = %v, want 650", f.PVToHouse)
	}
	if f.PVToBattery != 1800 {
		t.Errorf("pvToBattery = %v, want 1800", f.PVToBattery)
	}
	if f.PVToGrid != 3050 {
		t.Errorf("pvToGrid = %v, want 3050", f.PVToGrid)
	}
}

func TestNormalizeDay(t *testing.T) {
	raw := json.RawMessage(`[
	  {"timeStr":"2026-10-18 10:00:00","pac":2.1,"pacStr":"kW","familyLoadPower":0.6,"familyLoadPowerStr":"kW","psum":1.4,"psumStr":"kW","batteryCapacitySoc":55},
	  {"timeStr":"2026-10-18 10:05:00","pac":2.3,"pacStr":"kW","familyLoadPower":0.6,"familyLoadPowerStr":"kW","psum":1.6,"psumStr":"kW","batteryCapacitySoc":56}
	]`)
	loc := time.FixedZone("PKT", 5*3600)
	rows, err := NormalizeDay(raw, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].SolarW == nil || *rows[0].SolarW != 2100 {
		t.Errorf("solar = %v, want 2100", rows[0].SolarW)
	}
	if !rows[0].TS.Before(rows[1].TS) {
		t.Error("rows must be sorted by time")
	}
}

func TestNormalizeMonthAndYear(t *testing.T) {
	loc := time.FixedZone("PKT", 5*3600)
	monthRaw := json.RawMessage(`[
	  {"dateStr":"2026-10-01","energy":21.4,"energyStr":"kWh","homeLoadEnergy":13.0,"gridPurchasedEnergy":1.2,"gridSellEnergy":4.4,"money":577.8,"moneyStr":"PKR"}
	]`)
	days, err := NormalizeMonth(monthRaw, loc, "PKR")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("days = %d, want 1", len(days))
	}
	if days[0].GeneratedKWh == nil || *days[0].GeneratedKWh != 21.4 {
		t.Errorf("generated = %v, want 21.4", days[0].GeneratedKWh)
	}
	if days[0].Currency != "PKR" {
		t.Errorf("currency = %q", days[0].Currency)
	}

	yearRaw := json.RawMessage(`[{"dateStr":"2026-01","energy":866.0,"energyStr":"MWh","money":359.5,"moneyStr":"PKR"}]`)
	months, err := NormalizeYear(yearRaw, loc, "PKR")
	if err != nil {
		t.Fatal(err)
	}
	if len(months) != 1 {
		t.Fatalf("months = %d, want 1", len(months))
	}
	if months[0].GeneratedKWh == nil || *months[0].GeneratedKWh != 866000 {
		t.Errorf("generated = %v, want 866000", months[0].GeneratedKWh)
	}
}

func TestNormalizeAlarms(t *testing.T) {
	loc := time.FixedZone("PKT", 5*3600)
	raw := json.RawMessage(`{"page":{"records":[
	  {"id":"1","alarmCode":"1030","alarmLevel":"2","alarmMsg":"Grid voltage high","advice":"Wait","state":"2","alarmBeginTime":1760000000000,"alarmEndTime":1760001000000}
	]}}`)
	rows, err := NormalizeAlarms(raw, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("alarms = %d, want 1", len(rows))
	}
	if rows[0].Level != 2 || rows[0].Message != "Grid voltage high" {
		t.Errorf("alarm = %+v", rows[0])
	}
	if rows[0].Active {
		t.Error("restored alarm must not be active")
	}
}

func TestDayWattsDespiteKWUnit(t *testing.T) {
	// Some inverterDay rows carry watts while the unit says kW; a 2400 value
	// must stay 2400 W, not become 2.4 MW.
	raw := json.RawMessage(`[
	  {"timeStr":"2026-10-18 12:00:00","pac":2400,"pacStr":"kW","familyLoadPower":656,"familyLoadPowerStr":"kW","psum":666,"psumStr":"kW","batteryPower":0,"batteryPowerStr":"kW"}
	]`)
	rows, err := NormalizeDay(raw, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].SolarW == nil || *rows[0].SolarW != 2400 {
		t.Errorf("solar = %v, want 2400", rows[0].SolarW)
	}
	if rows[0].LoadW == nil || *rows[0].LoadW != 656 {
		t.Errorf("load = %v, want 656", rows[0].LoadW)
	}
	if rows[0].GridW == nil || *rows[0].GridW != 666 {
		t.Errorf("grid = %v, want 666", rows[0].GridW)
	}
}

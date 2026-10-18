package solis

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

type fakeServer struct {
	mu       sync.Mutex
	requests []string
	bodies   []string
	handler  func(w http.ResponseWriter, r *http.Request)
}

func newFakeServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *fakeServer) {
	t.Helper()
	fs := &fakeServer{handler: handler}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fs.mu.Lock()
		fs.requests = append(fs.requests, r.URL.Path)
		fs.bodies = append(fs.bodies, string(body))
		fs.mu.Unlock()

		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-MD5") == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "API ") {
			http.Error(w, "unsigned", http.StatusUnauthorized)
			return
		}
		if _, err := time.Parse(http.TimeFormat, r.Header.Get("Date")); err != nil {
			http.Error(w, "bad date", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fs.handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, fs
}

func writeEnvelope(w http.ResponseWriter, data string) {
	_, _ = w.Write([]byte(`{"success":true,"code":"0","msg":"success","data":` + data + `}`))
}

func TestClientSnapshotDiscoversInverter(t *testing.T) {
	srv, fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/api/inverterList":
			writeEnvelope(w, `{"page":{"records":[{"id":"42","sn":"SN42"}]}}`)
		case "/v1/api/inverterDetail":
			writeEnvelope(w, `{"sn":"SN42","stationName":"Home","state":1,"pac":5.0,"pacStr":"kW"}`)
		default:
			http.NotFound(w, r)
		}
	})

	client, err := New(Options{
		BaseURL: srv.URL, APIKey: "k", APISecret: "s", StationID: "7",
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.SolarW == nil || *snap.SolarW != 5000 {
		t.Errorf("solar = %v, want 5000", snap.SolarW)
	}
	if client.InverterID() != "42" {
		t.Errorf("inverter id = %q, want 42", client.InverterID())
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.requests) != 2 || fs.requests[0] != "/v1/api/inverterList" {
		t.Fatalf("requests = %v", fs.requests)
	}
	if fs.bodies[1] != `{"id":"42"}` {
		t.Errorf("detail body = %q", fs.bodies[1])
	}
}

func TestClientAPIError(t *testing.T) {
	srv, _ := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"code":"1001","msg":"invalid key"}`))
	})
	client, err := New(Options{BaseURL: srv.URL, APIKey: "k", APISecret: "s", InverterID: "1", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Snapshot(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Code != "1001" || apiErr.Message != "invalid key" {
		t.Errorf("api error = %+v", apiErr)
	}
}

func TestClientClockSkew(t *testing.T) {
	srv, _ := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "request timeout", http.StatusRequestTimeout)
	})
	client, err := New(Options{BaseURL: srv.URL, APIKey: "k", APISecret: "s", InverterID: "1", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Snapshot(context.Background())
	if !errors.Is(err, ErrClockSkew) {
		t.Fatalf("err = %v, want ErrClockSkew", err)
	}
}

func TestClientMalformedJSON(t *testing.T) {
	srv, _ := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"code":"0","data":`))
	})
	client, err := New(Options{BaseURL: srv.URL, APIKey: "k", APISecret: "s", InverterID: "1", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Snapshot(context.Background()); err == nil {
		t.Fatal("want a decode error")
	}
}

func TestClientMonthRequestBody(t *testing.T) {
	srv, fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, `[{"dateStr":"2026-10-01","energy":20,"energyStr":"kWh"}]`)
	})
	loc := time.FixedZone("PKT", 5*3600)
	client, err := New(Options{
		BaseURL: srv.URL, APIKey: "k", APISecret: "s", InverterID: "1",
		Currency: "PKR", Location: loc, HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.Month(context.Background(), 2026, time.October)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GeneratedKWh == nil || *rows[0].GeneratedKWh != 20 {
		t.Fatalf("rows = %+v", rows)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var body map[string]any
	if err := json.Unmarshal([]byte(fs.bodies[0]), &body); err != nil {
		t.Fatal(err)
	}
	if body["month"] != "2026-10" || body["money"] != "PKR" || body["timeZone"] != float64(5) {
		t.Errorf("month body = %v", body)
	}
}

func TestClientAlarmsPath(t *testing.T) {
	srv, fs := newFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(w, `{"page":{"records":[{"id":"1","alarmCode":"9","alarmLevel":"1","alarmMsg":"m"}]}}`)
	})
	client, err := New(Options{BaseURL: srv.URL, APIKey: "k", APISecret: "s", InverterID: "1", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.Alarms(context.Background(), time.Now().AddDate(0, 0, -30), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("alarms = %d", len(rows))
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.requests[0] != "/v1/api/alarmList" {
		t.Errorf("path = %q", fs.requests[0])
	}
}

var _ provider.Source = (*Client)(nil)

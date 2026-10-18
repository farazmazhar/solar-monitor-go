// Package solis implements provider.Source against the SolisCloud REST API.
package solis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

const (
	defaultBaseURL = "https://www.soliscloud.com:13333"
	maxBodyBytes   = 16 << 20
)

// ErrClockSkew is returned for HTTP 408, which SolisCloud uses when the
// request Date is more than 15 minutes away from server time.
var ErrClockSkew = errors.New("solis: clock skew (HTTP 408); check the system clock")

// APIError is a non-transport failure: a non-200 status or a non-zero code.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Status != 0 && e.Status != http.StatusOK {
		return fmt.Sprintf("solis: HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("solis: api code %s: %s", e.Code, e.Message)
}

// Options configures a Client.
type Options struct {
	BaseURL         string
	APIKey          string
	APISecret       string
	StationID       string
	InverterID      string
	InverterSN      string
	Timeout         time.Duration
	Currency        string
	GridPositive    string // export (default) or import
	BatteryPositive string // charge (default) or discharge
	Location        *time.Location
	HTTPClient      *http.Client
	Now             func() time.Time
}

// Client is a SolisCloud API client.
type Client struct {
	baseURL         string
	apiKey          string
	apiSecret       string
	stationID       string
	currency        string
	gridPositive    string
	batteryPositive string
	location        *time.Location
	now             func() time.Time
	http            *http.Client
	ownsHTTP        bool

	mu         sync.Mutex
	inverterID string
	inverterSN string
}

// New validates options and returns a client.
func New(opts Options) (*Client, error) {
	if opts.APIKey == "" || opts.APISecret == "" {
		return nil, errors.New("solis: api key and secret are required")
	}
	if opts.StationID == "" && opts.InverterID == "" && opts.InverterSN == "" {
		return nil, errors.New("solis: station id, inverter id or inverter sn is required")
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	loc := opts.Location
	if loc == nil {
		loc = time.Local
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	hc := opts.HTTPClient
	owns := false
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
		owns = true
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	grid := opts.GridPositive
	if grid == "" {
		grid = "export"
	}
	bat := opts.BatteryPositive
	if bat == "" {
		bat = "charge"
	}
	return &Client{
		baseURL:         base,
		apiKey:          opts.APIKey,
		apiSecret:       opts.APISecret,
		stationID:       opts.StationID,
		currency:        opts.Currency,
		gridPositive:    grid,
		batteryPositive: bat,
		location:        loc,
		now:             now,
		http:            hc,
		ownsHTTP:        owns,
		inverterID:      opts.InverterID,
		inverterSN:      opts.InverterSN,
	}, nil
}

// Name implements provider.Source.
func (c *Client) Name() string { return "solis" }

// Close implements provider.Source.
func (c *Client) Close() error {
	if c.ownsHTTP {
		c.http.CloseIdleConnections()
	}
	return nil
}

// BaseURL returns the configured API base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// GridPositive and BatteryPositive expose the sign conventions in force.
func (c *Client) GridPositive() string    { return c.gridPositive }
func (c *Client) BatteryPositive() string { return c.batteryPositive }

// InverterID returns the resolved inverter id, if known.
func (c *Client) InverterID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inverterID
}

// ResetInverter forgets the cached inverter id and serial so the next call
// discovers them again.
func (c *Client) ResetInverter() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inverterID = ""
	c.inverterSN = ""
}

// Snapshot implements provider.Source.
func (c *Client) Snapshot(ctx context.Context) (provider.Snapshot, error) {
	id, err := c.ensureID(ctx)
	if err != nil {
		return provider.Snapshot{}, err
	}
	raw, err := c.post(ctx, "/v1/api/inverterDetail", map[string]any{"id": id})
	if err != nil {
		return provider.Snapshot{}, err
	}
	snap, err := NormalizeSnapshot(raw, NormalizeOptions{
		Now:             c.now(),
		Location:        c.location,
		Currency:        c.currency,
		GridPositive:    c.gridPositive,
		BatteryPositive: c.batteryPositive,
		Source:          "solis",
	})
	if err != nil {
		return provider.Snapshot{}, err
	}
	if snap.SN == "" {
		c.mu.Lock()
		snap.SN = c.inverterSN
		c.mu.Unlock()
	}
	return snap, nil
}

// RawDetail returns the unmodified inverterDetail payload. It backs --discover.
func (c *Client) RawDetail(ctx context.Context) (json.RawMessage, error) {
	id, err := c.ensureID(ctx)
	if err != nil {
		return nil, err
	}
	return c.post(ctx, "/v1/api/inverterDetail", map[string]any{"id": id})
}

// Today implements provider.Source.
func (c *Client) Today(ctx context.Context, day time.Time) ([]provider.Sample, error) {
	id, err := c.ensureID(ctx)
	if err != nil {
		return nil, err
	}
	local := day.In(c.location)
	body := map[string]any{
		"id":       id,
		"money":    c.currency,
		"time":     local.Format("2006-01-02"),
		"timeZone": c.offsetHours(local),
	}
	raw, err := c.post(ctx, "/v1/api/inverterDay", body)
	if err != nil {
		return nil, err
	}
	return NormalizeDay(raw, c.location)
}

// Month implements provider.Source.
func (c *Client) Month(ctx context.Context, year int, month time.Month) ([]provider.DayEnergy, error) {
	id, err := c.ensureID(ctx)
	if err != nil {
		return nil, err
	}
	local := time.Date(year, month, 1, 12, 0, 0, 0, c.location)
	body := map[string]any{
		"id":       id,
		"money":    c.currency,
		"month":    local.Format("2006-01"),
		"timeZone": c.offsetHours(local),
	}
	raw, err := c.post(ctx, "/v1/api/inverterMonth", body)
	if err != nil {
		return nil, err
	}
	return NormalizeMonth(raw, c.location, c.currency)
}

// Year implements provider.Source.
func (c *Client) Year(ctx context.Context, year int) ([]provider.MonthEnergy, error) {
	id, err := c.ensureID(ctx)
	if err != nil {
		return nil, err
	}
	local := time.Date(year, time.January, 1, 12, 0, 0, 0, c.location)
	body := map[string]any{
		"id":       id,
		"money":    c.currency,
		"year":     local.Format("2006"),
		"timeZone": c.offsetHours(local),
	}
	raw, err := c.post(ctx, "/v1/api/inverterYear", body)
	if err != nil {
		return nil, err
	}
	return NormalizeYear(raw, c.location, c.currency)
}

// Alarms implements provider.Source.
func (c *Client) Alarms(ctx context.Context, from, to time.Time) ([]provider.Alarm, error) {
	body := map[string]any{
		"pageNo":         "1",
		"pageSize":       "100",
		"alarmBeginTime": from.In(c.location).Format("2006-01-02"),
		"alarmEndTime":   to.In(c.location).Format("2006-01-02"),
	}
	c.mu.Lock()
	sn := c.inverterSN
	c.mu.Unlock()
	if sn != "" {
		body["alarmDeviceSn"] = sn
	}

	raw, err := c.post(ctx, "/v1/api/alarmList", body)
	if err == nil {
		return NormalizeAlarms(raw, c.location)
	}

	// A station id that the account cannot see, or an unusual device scope,
	// shows up as the generic API code 1. Retry once account-wide and filter
	// to the inverter locally.
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "1" {
		return nil, err
	}
	delete(body, "alarmDeviceSn")
	raw, retryErr := c.post(ctx, "/v1/api/alarmList", body)
	if retryErr != nil {
		return nil, err
	}
	alarms, nerr := NormalizeAlarms(raw, c.location)
	if nerr != nil {
		return nil, nerr
	}
	if sn != "" {
		alarms = filterAlarmsBySN(alarms, sn)
	}
	return alarms, nil
}

func filterAlarmsBySN(alarms []provider.Alarm, sn string) []provider.Alarm {
	out := alarms[:0]
	for _, a := range alarms {
		if a.DeviceSN == "" || strings.EqualFold(a.DeviceSN, sn) {
			out = append(out, a)
		}
	}
	return out
}

func (c *Client) offsetHours(t time.Time) int {
	t = t.In(c.location)
	_, off := t.Zone()
	return off / 3600
}

func (c *Client) ensureID(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.inverterID != "" {
		id := c.inverterID
		c.mu.Unlock()
		return id, nil
	}
	if c.inverterSN != "" {
		// The API accepts a serial number in the id field, so a configured SN
		// is enough without a separate discovery round trip.
		sn := c.inverterSN
		c.inverterID = sn
		c.mu.Unlock()
		return sn, nil
	}
	c.mu.Unlock()

	if c.stationID == "" {
		return "", errors.New("solis: no inverter id and no station id to discover one")
	}
	// The current API expects stationId, but older deployments accepted id;
	// try the documented field first and fall back for compatibility.
	raw, err := c.post(ctx, "/v1/api/inverterList", map[string]any{
		"stationId": c.stationID,
		"pageNo":    1,
		"pageSize":  10,
	})
	if err != nil {
		raw, err = c.post(ctx, "/v1/api/inverterList", map[string]any{
			"id":       c.stationID,
			"pageNo":   1,
			"pageSize": 10,
		})
	}
	if err != nil {
		return "", fmt.Errorf("solis: discover inverter: %w", err)
	}
	id, sn, err := firstInverter(raw)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.inverterID = id
	if sn != "" {
		c.inverterSN = sn
	}
	c.mu.Unlock()
	return id, nil
}

func firstInverter(raw json.RawMessage) (id, sn string, err error) {
	var payload struct {
		Page struct {
			Records []struct {
				ID string `json:"id"`
				SN string `json:"sn"`
			} `json:"records"`
		} `json:"page"`
		Records []struct {
			ID string `json:"id"`
			SN string `json:"sn"`
		} `json:"records"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", fmt.Errorf("solis: decode inverter list: %w", err)
	}
	records := payload.Page.Records
	if len(records) == 0 {
		records = payload.Records
	}
	if len(records) == 0 {
		return "", "", errors.New("solis: no inverters found for this station")
	}
	return records[0].ID, records[0].SN, nil
}

func (c *Client) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("solis: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("solis: build request: %w", err)
	}

	now := c.now()
	md5b64, auth := SignRequest(c.apiKey, c.apiSecret, http.MethodPost, path, payload, now)
	req.Header.Set("Content-Type", ContentType())
	req.Header.Set("Date", now.UTC().Format(http.TimeFormat))
	req.Header.Set("Content-MD5", md5b64)
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("solis: request %s: %w", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("solis: read %s: %w", path, err)
	}
	if resp.StatusCode == http.StatusRequestTimeout {
		return nil, ErrClockSkew
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Status: resp.StatusCode, Message: truncate(string(data), 300)}
	}

	var env struct {
		Success bool            `json:"success"`
		Code    json.RawMessage `json:"code"`
		Msg     string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("solis: decode %s: %w", path, err)
	}
	code := codeString(env.Code)
	if !env.Success || code != "0" {
		return nil, &APIError{Code: code, Message: env.Msg}
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil, fmt.Errorf("solis: %s returned no data", path)
	}
	return env.Data, nil
}

func codeString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return strings.Trim(string(raw), `"`)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

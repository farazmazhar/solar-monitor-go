package ui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/farazmazhar/solar-moonitor-go/internal/config"
	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

// Tab identifies a top-level screen.
type Tab int

const (
	TabLive Tab = iota
	TabToday
	TabHistory
	TabBattery
	TabPV
	TabAlarms
	TabDiagnostics
)

var tabTitles = []string{"Live", "Today", "History", "Battery", "PV", "Alarms", "Diagnostics"}

// HistoryMode selects the month or year view.
type HistoryMode int

const (
	HistoryMonth HistoryMode = iota
	HistoryYear
)

// AlarmFilter selects which alarms are listed.
type AlarmFilter int

const (
	AlarmActive AlarmFilter = iota
	AlarmRecent
	AlarmAll
)

// StoreStats is a provider-neutral summary of the local history store.
type StoreStats struct {
	Path      string
	SizeBytes int64
	Samples   int
	Oldest    time.Time
	Newest    time.Time
}

// Options wires the UI to its data source.
type Options struct {
	Config       config.Config
	Paths        config.Paths
	Source       provider.Source
	Demo         bool
	Version      string
	SetupErr     error
	DemoFactory  func() provider.Source
	InitialTab   Tab
	SeedSnapshot *provider.Snapshot
	SeedSamples  []provider.Sample
	Stats        func() (StoreStats, bool)
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg      config.Config
	paths    config.Paths
	th       Theme
	src      provider.Source
	demo     bool
	version  string
	setupErr error

	demoFactory func() provider.Source

	width, height int
	tab           Tab
	help          bool
	motion        bool
	phase         int
	hits          HitRegistry

	snap        provider.Snapshot
	haveSnap    bool
	lastLatency time.Duration
	lastErr     error
	polling     bool
	nextPoll    time.Time
	notice      string

	daySamples []provider.Sample
	day        time.Time
	dayErr     error

	historyMode HistoryMode
	month       []provider.DayEnergy
	monthAt     time.Time
	monthErr    error
	year        []provider.MonthEnergy
	yearAt      time.Time
	yearErr     error

	alarms      []provider.Alarm
	alarmsAt    time.Time
	alarmsErr   error
	alarmFilter AlarmFilter
	alarmSel    int

	chartCursor int
	series      [4]bool
	pvScroll    int
	rawScroll   int
	exportMsg   string
	exportErr   error

	// clock is injectable so golden tests can pin the wall clock.
	clock func() time.Time

	stats func() (StoreStats, bool)
}

// now returns the wall clock, or a pinned clock in tests.
func (m *Model) now() time.Time {
	if m.clock == nil {
		return time.Now()
	}
	return m.clock()
}

type (
	snapshotMsg struct {
		snap    provider.Snapshot
		err     error
		latency time.Duration
	}
	todayMsg struct {
		day     time.Time
		samples []provider.Sample
		err     error
	}
	monthMsg struct {
		at   time.Time
		rows []provider.DayEnergy
		err  error
	}
	yearMsg struct {
		year int
		rows []provider.MonthEnergy
		err  error
	}
	alarmsMsg struct {
		at     time.Time
		alarms []provider.Alarm
		err    error
	}
	clockMsg   struct{}
	refreshMsg struct{}
	exportMsg  struct {
		path string
		err  error
	}
)

// New builds a model. A nil Source with a nil SetupErr becomes a setup error.
func New(opts Options) *Model {
	if opts.Source == nil && opts.SetupErr == nil {
		opts.SetupErr = errors.New("no data source configured")
	}
	m := &Model{
		cfg:         opts.Config,
		paths:       opts.Paths,
		th:          NewTheme(opts.Config.Display.Theme),
		src:         opts.Source,
		demo:        opts.Demo,
		version:     opts.Version,
		setupErr:    opts.SetupErr,
		demoFactory: opts.DemoFactory,
		width:       80,
		height:      24,
		tab:         opts.InitialTab,
		motion:      opts.Config.Display.Motion,
		chartCursor: -1,
		historyMode: HistoryMonth,
		alarmFilter: AlarmRecent,
		clock:       time.Now,
		stats:       opts.Stats,
	}
	if opts.SeedSnapshot != nil {
		m.snap = *opts.SeedSnapshot
		m.haveSnap = true
	}
	if len(opts.SeedSamples) > 0 {
		m.daySamples = opts.SeedSamples
	}
	m.day = m.now()
	m.monthAt = firstOfMonth(m.now())
	m.yearAt = time.Date(m.now().Year(), 1, 1, 0, 0, 0, 0, time.Local)
	m.nextPoll = m.now()
	for i := range m.series {
		m.series[i] = true
	}
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	if m.src == nil {
		return clockCmd()
	}
	m.polling = true
	m.nextPoll = m.now().Add(m.pollDelay())
	return tea.Batch(
		m.fetchSnapshotCmd(),
		m.fetchTodayCmd(),
		m.fetchMonthCmd(m.monthAt),
		m.fetchYearCmd(m.yearAt.Year()),
		m.fetchAlarmsCmd(),
		clockCmd(),
	)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case clockMsg:
		if m.motion && m.tab == TabLive {
			m.phase++
		}
		var cmds []tea.Cmd
		if m.src != nil && !m.polling && !m.now().Before(m.nextPoll) {
			m.polling = true
			m.nextPoll = m.now().Add(m.pollDelay())
			cmds = append(cmds, m.fetchSnapshotCmd(), m.fetchTodayCmd())
			if time.Since(m.alarmsAt) >= m.cfg.AlarmsInterval() {
				cmds = append(cmds, m.fetchAlarmsCmd())
			}
		}
		cmds = append(cmds, clockCmd())
		return m, tea.Batch(cmds...)

	case snapshotMsg:
		m.polling = false
		m.lastLatency = msg.latency
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.snap = msg.snap
		m.haveSnap = true
		m.lastErr = nil
		m.appendSample(msg.snap)
		return m, nil

	case todayMsg:
		if sameDay(msg.day, m.day) || m.day.IsZero() {
			m.day = msg.day
			if msg.err != nil {
				m.dayErr = msg.err
				return m, nil
			}
			m.dayErr = nil
			m.daySamples = msg.samples
			m.chartCursor = clamp(m.chartCursor, -1, len(m.daySamples)-1)
		}
		return m, nil

	case monthMsg:
		m.month = msg.rows
		m.monthAt = msg.at
		m.monthErr = msg.err
		return m, nil

	case yearMsg:
		m.year = msg.rows
		m.yearAt = time.Date(msg.year, 1, 1, 0, 0, 0, 0, time.Local)
		m.yearErr = msg.err
		return m, nil

	case alarmsMsg:
		m.alarms = msg.alarms
		m.alarmsAt = msg.at
		m.alarmsErr = msg.err
		m.alarmSel = clamp(m.alarmSel, 0, max(0, len(m.filteredAlarms())-1))
		return m, nil

	case refreshMsg:
		if m.src == nil {
			return m, nil
		}
		m.polling = true
		m.nextPoll = m.now().Add(m.pollDelay())
		m.notice = ""
		return m, tea.Batch(
			m.fetchSnapshotCmd(),
			m.fetchTodayCmd(),
			m.fetchAlarmsCmd(),
			m.fetchMonthCmd(m.monthAt),
			m.fetchYearCmd(m.yearAt.Year()),
		)

	case exportMsg:
		if msg.err != nil {
			m.exportErr = msg.err
			m.exportMsg = ""
		} else {
			m.exportErr = nil
			m.exportMsg = "wrote " + msg.path
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg:
		return m.handleClick(msg.Mouse())

	case tea.MouseWheelMsg:
		return m.handleWheel(msg.Mouse())
	}
	return m, nil
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	m.hits.Reset()
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}

	var content string
	if m.help {
		content = m.renderHelp(w, h)
	} else if m.setupErr != nil {
		content = m.renderSetup(w, h)
	} else {
		header := m.renderHeader(w)
		tabs := m.renderTabs(w, 1)
		footer := m.renderFooter(w)
		bodyH := h - 3
		if bodyH < 6 {
			bodyH = 6
		}
		body := m.renderScreen(w, bodyH, 2)
		content = fitFrame(strings.Join([]string{header, tabs, body, footer}, "\n"), w, h)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "solar-moonitor-go"
	if m.th.Bg != nil {
		v.BackgroundColor = m.th.Bg
	}
	if m.th.Fg != nil {
		v.ForegroundColor = m.th.Fg
	}
	return v
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.help {
		switch key {
		case "esc", "?", "q":
			m.help = false
			return m, nil
		}
		return m, nil
	}
	if key == "?" {
		m.help = true
		return m, nil
	}
	if key == "esc" {
		m.notice = ""
		m.exportErr = nil
		m.exportMsg = ""
		return m, nil
	}
	if key == "q" {
		return m, tea.Quit
	}
	if m.setupErr != nil {
		if key == "d" && m.demoFactory != nil {
			m.startDemo()
			return m, tea.Batch(
				m.fetchSnapshotCmd(),
				m.fetchTodayCmd(),
				m.fetchMonthCmd(m.monthAt),
				m.fetchYearCmd(m.yearAt.Year()),
				m.fetchAlarmsCmd(),
				clockCmd(),
			)
		}
		return m, nil
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '7' {
		return m, m.switchTab(Tab(key[0] - '1'))
	}
	switch key {
	case "r":
		return m, func() tea.Msg { return refreshMsg{} }
	case "m":
		m.motion = !m.motion
		if m.motion {
			m.notice = "motion on"
		} else {
			m.notice = "motion off"
		}
		return m, nil
	case "t":
		m.day = m.now()
		m.chartCursor = -1
		if m.tab == TabHistory {
			m.monthAt = firstOfMonth(m.now())
			m.yearAt = time.Date(m.now().Year(), 1, 1, 0, 0, 0, 0, time.Local)
			return m, tea.Batch(m.fetchMonthCmd(m.monthAt), m.fetchYearCmd(m.yearAt.Year()))
		}
		return m, m.fetchTodayCmd()
	case "tab":
		switch m.tab {
		case TabHistory:
			if m.historyMode == HistoryMonth {
				m.historyMode = HistoryYear
			} else {
				m.historyMode = HistoryMonth
			}
		case TabAlarms:
			m.alarmFilter = (m.alarmFilter + 1) % 3
			m.alarmSel = clamp(m.alarmSel, 0, max(0, len(m.filteredAlarms())-1))
		default:
			return m, m.switchTab((m.tab + 1) % Tab(len(tabTitles)))
		}
		return m, nil
	case "shift+tab":
		return m, m.switchTab((m.tab + Tab(len(tabTitles)) - 1) % Tab(len(tabTitles)))
	}

	switch m.tab {
	case TabToday:
		switch key {
		case "left":
			m.moveCursor(-1)
		case "right":
			m.moveCursor(1)
		case "s":
			m.series[0] = !m.series[0]
		case "l":
			m.series[1] = !m.series[1]
		case "g":
			m.series[2] = !m.series[2]
		case "b":
			m.series[3] = !m.series[3]
		}
	case TabHistory:
		switch key {
		case "left", "h":
			if m.historyMode == HistoryMonth {
				m.monthAt = m.monthAt.AddDate(0, -1, 0)
				return m, m.fetchMonthCmd(m.monthAt)
			}
			m.yearAt = m.yearAt.AddDate(-1, 0, 0)
			return m, m.fetchYearCmd(m.yearAt.Year())
		case "right", "l":
			if m.historyMode == HistoryMonth {
				m.monthAt = m.monthAt.AddDate(0, 1, 0)
				return m, m.fetchMonthCmd(m.monthAt)
			}
			m.yearAt = m.yearAt.AddDate(1, 0, 0)
			return m, m.fetchYearCmd(m.yearAt.Year())
		case "e":
			return m, m.exportHistoryCmd()
		}
	case TabBattery:
		switch key {
		case "left", "h":
			m.moveCursor(-1)
		case "right", "l":
			m.moveCursor(1)
		}
	case TabPV:
		switch key {
		case "up", "k":
			m.pvScroll = max(0, m.pvScroll-1)
		case "down", "j":
			m.pvScroll = min(max(0, len(m.snap.PVStrings)-4), m.pvScroll+1)
		case "g":
			m.pvScroll = 0
		case "G":
			m.pvScroll = max(0, len(m.snap.PVStrings)-4)
		}
	case TabAlarms:
		switch key {
		case "up", "k":
			m.alarmSel = max(0, m.alarmSel-1)
		case "down", "j":
			n := len(m.filteredAlarms())
			if n > 0 {
				m.alarmSel = clamp(m.alarmSel+1, 0, n-1)
			}
		}
	case TabDiagnostics:
		switch key {
		case "up", "k":
			m.rawScroll = max(0, m.rawScroll-1)
		case "down", "j":
			m.rawScroll++
		case "g":
			m.rawScroll = 0
		case "e":
			return m, m.exportSnapshotCmd()
		}
	}
	return m, nil
}

func (m *Model) handleClick(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	action, ok := m.hits.At(mouse.X, mouse.Y)
	if !ok {
		return m, nil
	}
	switch action.Kind {
	case ActionTab:
		return m, m.switchTab(action.Tab)
	case ActionTile:
		switch action.Tile {
		case TileSolar:
			return m, m.switchTab(TabPV)
		case TileHouse, TileGrid:
			return m, m.switchTab(TabToday)
		case TileBattery:
			return m, m.switchTab(TabBattery)
		case TileIncome, TileCO2:
			return m, m.switchTab(TabHistory)
		}
	case ActionChartCursor:
		n := len(m.daySamples)
		if n > 1 && action.Rect.W > 0 {
			rel := clamp(mouse.X-action.Rect.X, 0, action.Rect.W-1)
			m.chartCursor = clamp(int(float64(rel)/float64(action.Rect.W-1)*float64(n-1)), 0, n-1)
		}
	case ActionAlarmRow:
		m.alarmSel = action.Row
	}
	return m, nil
}

func (m *Model) handleWheel(mouse tea.Mouse) (tea.Model, tea.Cmd) {
	switch m.tab {
	case TabPV:
		if mouse.Button == tea.MouseWheelUp {
			m.pvScroll = max(0, m.pvScroll-1)
		} else {
			m.pvScroll = min(max(0, len(m.snap.PVStrings)-4), m.pvScroll+1)
		}
	case TabDiagnostics:
		if mouse.Button == tea.MouseWheelUp {
			m.rawScroll = max(0, m.rawScroll-1)
		} else {
			m.rawScroll++
		}
	case TabAlarms:
		if mouse.Button == tea.MouseWheelUp {
			m.alarmSel = max(0, m.alarmSel-1)
		} else if n := len(m.filteredAlarms()); n > 0 {
			m.alarmSel = clamp(m.alarmSel+1, 0, n-1)
		}
	}
	return m, nil
}

func (m *Model) switchTab(t Tab) tea.Cmd {
	if t < 0 || int(t) >= len(tabTitles) {
		return nil
	}
	m.tab = t
	m.notice = ""
	switch t {
	case TabHistory:
		if len(m.month) == 0 && m.monthErr == nil {
			return m.fetchMonthCmd(m.monthAt)
		}
	case TabAlarms:
		if time.Since(m.alarmsAt) > m.cfg.AlarmsInterval() {
			return m.fetchAlarmsCmd()
		}
	case TabToday:
		if len(m.daySamples) == 0 && m.dayErr == nil {
			return m.fetchTodayCmd()
		}
	}
	return nil
}

// SetNotice sets the footer notice before the program starts.
func (m *Model) SetNotice(s string) { m.notice = s }

func (m *Model) moveCursor(delta int) {
	if len(m.daySamples) == 0 {
		return
	}
	if m.chartCursor < 0 {
		m.chartCursor = len(m.daySamples) - 1
		return
	}
	m.chartCursor = clamp(m.chartCursor+delta, 0, len(m.daySamples)-1)
}

func (m *Model) startDemo() {
	if m.demoFactory == nil {
		return
	}
	m.src = m.demoFactory()
	m.demo = true
	m.setupErr = nil
	m.cfg.Demo.Enabled = true
	m.polling = false
	m.nextPoll = m.now()
	m.notice = "demo mode: simulated data, in-memory only"
}

func (m *Model) appendSample(s provider.Snapshot) {
	if s.DataAt.IsZero() {
		return
	}
	sample := provider.Sample{
		TS:       s.DataAt,
		SolarW:   s.SolarW,
		LoadW:    s.LoadW,
		GridW:    s.GridW,
		BatteryW: s.BatteryW,
		SOC:      s.SOC,
		ImportW:  s.GridImportW,
		ExportW:  s.GridExportW,
	}
	if len(m.daySamples) > 0 {
		last := m.daySamples[len(m.daySamples)-1]
		if !sample.TS.After(last.TS) {
			m.daySamples[len(m.daySamples)-1] = sample
			return
		}
	}
	m.daySamples = append(m.daySamples, sample)
	if len(m.daySamples) > 2000 {
		m.daySamples = m.daySamples[len(m.daySamples)-2000:]
	}
}

func (m *Model) filteredAlarms() []provider.Alarm {
	var out []provider.Alarm
	for _, a := range m.alarms {
		switch m.alarmFilter {
		case AlarmActive:
			if !a.Active {
				continue
			}
		case AlarmRecent:
			if a.Begin.Before(m.now().AddDate(0, 0, -30)) {
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

func (m *Model) pollDelay() time.Duration {
	if m.demo {
		return 2 * time.Second
	}
	return m.cfg.PollIntervalDuration()
}

func (m *Model) fetchSnapshotCmd() tea.Cmd {
	src := m.src
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		start := time.Now()
		snap, err := src.Snapshot(ctx)
		return snapshotMsg{snap: snap, err: err, latency: time.Since(start)}
	}
}

func (m *Model) fetchTodayCmd() tea.Cmd {
	src := m.src
	day := m.day
	if day.IsZero() {
		day = m.now()
	}
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		samples, err := src.Today(ctx, day)
		return todayMsg{day: day, samples: samples, err: err}
	}
}

func (m *Model) fetchMonthCmd(at time.Time) tea.Cmd {
	src := m.src
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		rows, err := src.Month(ctx, at.Year(), at.Month())
		return monthMsg{at: firstOfMonth(at), rows: rows, err: err}
	}
}

func (m *Model) fetchYearCmd(year int) tea.Cmd {
	src := m.src
	if src == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		rows, err := src.Year(ctx, year)
		return yearMsg{year: year, rows: rows, err: err}
	}
}

func (m *Model) fetchAlarmsCmd() tea.Cmd {
	src := m.src
	if src == nil {
		return nil
	}
	now := m.now()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		rows, err := src.Alarms(ctx, now.AddDate(0, 0, -30), now)
		return alarmsMsg{at: now, alarms: rows, err: err}
	}
}

func clockCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return clockMsg{} })
}

func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func (m *Model) selectedSample() *provider.Sample {
	if m.chartCursor < 0 || m.chartCursor >= len(m.daySamples) {
		if len(m.daySamples) == 0 {
			return nil
		}
		return &m.daySamples[len(m.daySamples)-1]
	}
	return &m.daySamples[m.chartCursor]
}

func (m *Model) lastRefreshLabel() string {
	if !m.haveSnap {
		return "waiting for data"
	}
	return "updated " + fmtAge(m.snap.FetchedAt) + " ago"
}

func (m *Model) errorText() string {
	var parts []string
	if m.lastErr != nil {
		parts = append(parts, m.lastErr.Error())
	}
	if m.dayErr != nil {
		parts = append(parts, "today: "+m.dayErr.Error())
	}
	if m.alarmsErr != nil && m.tab == TabAlarms {
		parts = append(parts, "alarms: "+m.alarmsErr.Error())
	}
	return strings.Join(parts, " · ")
}

func (m *Model) fetchActiveHistory() tea.Cmd {
	if m.historyMode == HistoryYear {
		return m.fetchYearCmd(m.yearAt.Year())
	}
	return m.fetchMonthCmd(m.monthAt)
}

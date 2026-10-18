package ui

// ActionKind describes what a mouse hit does.
type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionTab
	ActionTile
	ActionChartCursor
	ActionAlarmRow
)

// Tile identifies a Live tile.
type Tile int

const (
	TileSolar Tile = iota
	TileHouse
	TileGrid
	TileBattery
	TileIncome
	TileCO2
)

// Action is registered for a screen rectangle.
type Action struct {
	Kind ActionKind
	Tab  Tab
	Tile Tile
	Row  int
	Rect Rect
}

type hitEntry struct {
	rect   Rect
	action Action
}

// Rect is a screen rectangle in cells.
type Rect struct {
	X, Y, W, H int
}

// HitRegistry maps rendered rectangles back to actions. View populates it each
// frame; Update reads the most recent frame.
type HitRegistry struct {
	entries []hitEntry
}

// Reset clears the registry at the start of a render.
func (r *HitRegistry) Reset() { r.entries = r.entries[:0] }

// Add registers a rectangle. Empty rectangles are ignored.
func (r *HitRegistry) Add(rect Rect, action Action) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	r.entries = append(r.entries, hitEntry{rect: rect, action: action})
}

// At returns the topmost action containing the point.
func (r *HitRegistry) At(x, y int) (Action, bool) {
	for i := len(r.entries) - 1; i >= 0; i-- {
		e := r.entries[i]
		if x >= e.rect.X && x < e.rect.X+e.rect.W && y >= e.rect.Y && y < e.rect.Y+e.rect.H {
			return e.action, true
		}
	}
	return Action{}, false
}

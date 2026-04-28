package game

type Planet struct {
	ID     int     `json:"id"     cbor:"id"`
	X      float64 `json:"x"      cbor:"x"`
	Y      float64 `json:"y"      cbor:"y"`
	Radius float64 `json:"r"      cbor:"r"`
	Owner  int     `json:"owner"  cbor:"owner"`
	Ships  int     `json:"ships"  cbor:"ships"`
	Growth int     `json:"growth" cbor:"growth"`
}

type Fleet struct {
	ID             int     `json:"id"    cbor:"id"`
	Owner          int     `json:"owner" cbor:"owner"`
	SourceID       int     `json:"src"   cbor:"src"`
	TargetID       int     `json:"dst"   cbor:"dst"`
	Ships          int     `json:"ships" cbor:"ships"`
	LaunchTick     int64   `json:"-"     cbor:"-"`
	ETA            int64   `json:"-"     cbor:"-"`
	X              float64 `json:"x"     cbor:"x"`
	Y              float64 `json:"y"     cbor:"y"`
	VX             float64 `json:"vx"    cbor:"vx"`
	VY             float64 `json:"vy"    cbor:"vy"`
	AvoidPlanetID  int     `json:"-"     cbor:"-"`
	AvoidClockwise bool    `json:"-"     cbor:"-"`
}

type PlayerColor struct {
	PlayerID int `json:"playerId" cbor:"playerId"`
	Color    int `json:"color"    cbor:"color"`
}

type Snapshot struct {
	Tick         int64         `json:"tick"         cbor:"tick"`
	TickRate     int           `json:"tickRate"     cbor:"tickRate"`
	Width        float64       `json:"width"        cbor:"width"`
	Height       float64       `json:"height"       cbor:"height"`
	Planets      []Planet      `json:"planets"      cbor:"planets"`
	Fleets       []Fleet       `json:"fleets"       cbor:"fleets"`
	PlayerColors []PlayerColor `json:"playerColors" cbor:"playerColors"`
}

package game

type Planet struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"r"`
	Owner  int     `json:"owner"`
	Ships  int     `json:"ships"`
	Growth int     `json:"growth"`
}

type Fleet struct {
	ID             int     `json:"id"`
	Owner          int     `json:"owner"`
	SourceID       int     `json:"src"`
	TargetID       int     `json:"dst"`
	Ships          int     `json:"ships"`
	LaunchTick     int64   `json:"-"`
	ETA            int64   `json:"-"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	VX             float64 `json:"vx"`
	VY             float64 `json:"vy"`
	AvoidPlanetID  int     `json:"-"`
	AvoidClockwise bool    `json:"-"`
}

type PlayerColor struct {
	PlayerID int `json:"playerId"`
	Color    int `json:"color"`
}

type Snapshot struct {
	Tick         int64         `json:"tick"`
	TickRate     int           `json:"tickRate"`
	Width        float64       `json:"width"`
	Height       float64       `json:"height"`
	Planets      []Planet      `json:"planets"`
	Fleets       []Fleet       `json:"fleets"`
	PlayerColors []PlayerColor `json:"playerColors"`
}

package server

import "github.com/yyewolf/gcg/internal/game"

type clientCommand struct {
	Type   string `json:"t"`
	Lobby  string `json:"lobby,omitempty"`
	Source int    `json:"src"`
	Target int    `json:"dst"`
	Pct    int    `json:"pct"`
}

type lobbySummary struct {
	ID          string `json:"id"`
	Players     int    `json:"players"`
	MaxPlayers  int    `json:"maxPlayers"`
	Status      string `json:"status"`
	CountdownMS int64  `json:"countdownMs,omitempty"`
}

type outboundMessage struct {
	Type          string         `json:"t"`
	Player        int            `json:"playerId,omitempty"`
	Winner        int            `json:"winnerId,omitempty"`
	JoinedLobbyID string         `json:"joinedLobbyId,omitempty"`
	LobbyStatus   string         `json:"lobbyStatus,omitempty"`
	LobbyPlayers  int            `json:"lobbyPlayers,omitempty"`
	CountdownMS   int64          `json:"countdownMs,omitempty"`
	Lobbies       []lobbySummary `json:"lobbies,omitempty"`
	Tick          int64          `json:"tick"`
	TickRate      int            `json:"tickRate,omitempty"`
	MapName       string         `json:"map,omitempty"`
	State         *game.Snapshot `json:"state,omitempty"`
	Error         string         `json:"error,omitempty"`
}

type compactStateMessage struct {
	Type     string       `json:"t"`
	Tick     int64        `json:"tick"`
	TickRate int          `json:"r"`
	Planets  [][3]int     `json:"p"`
	Fleets   [][9]float64 `json:"f"`
}

func newCompactStateMessage(snapshot game.Snapshot) compactStateMessage {
	planets := make([][3]int, 0, len(snapshot.Planets))
	for _, planet := range snapshot.Planets {
		planets = append(planets, [3]int{planet.ID, planet.Owner, planet.Ships})
	}

	fleets := make([][9]float64, 0, len(snapshot.Fleets))
	for _, fleet := range snapshot.Fleets {
		fleets = append(fleets, [9]float64{
			float64(fleet.ID),
			float64(fleet.Owner),
			float64(fleet.SourceID),
			float64(fleet.TargetID),
			float64(fleet.Ships),
			fleet.X,
			fleet.Y,
			fleet.VX,
			fleet.VY,
		})
	}

	return compactStateMessage{
		Type:     "state",
		Tick:     snapshot.Tick,
		TickRate: snapshot.TickRate,
		Planets:  planets,
		Fleets:   fleets,
	}
}

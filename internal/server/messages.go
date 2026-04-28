package server

import "github.com/yyewolf/gcg/internal/game"

type clientCommand struct {
	Type   string `json:"t"              cbor:"t"`
	Lobby  string `json:"lobby,omitempty" cbor:"lobby,omitempty"`
	Source int    `json:"src"            cbor:"src"`
	Target int    `json:"dst"            cbor:"dst"`
	Pct    int    `json:"pct"            cbor:"pct"`
}

type lobbySummary struct {
	ID          string `json:"id"                    cbor:"id"`
	Players     int    `json:"players"               cbor:"players"`
	MaxPlayers  int    `json:"maxPlayers"             cbor:"maxPlayers"`
	Status      string `json:"status"                cbor:"status"`
	CountdownMS int64  `json:"countdownMs,omitempty" cbor:"countdownMs,omitempty"`
}

type outboundMessage struct {
	Type          string         `json:"t"                     cbor:"t"`
	Player        int            `json:"playerId,omitempty"    cbor:"playerId,omitempty"`
	Winner        int            `json:"winnerId,omitempty"    cbor:"winnerId,omitempty"`
	JoinedLobbyID string         `json:"joinedLobbyId,omitempty" cbor:"joinedLobbyId,omitempty"`
	LobbyStatus   string         `json:"lobbyStatus,omitempty" cbor:"lobbyStatus,omitempty"`
	LobbyPlayers  int            `json:"lobbyPlayers,omitempty" cbor:"lobbyPlayers,omitempty"`
	CountdownMS   int64          `json:"countdownMs,omitempty" cbor:"countdownMs,omitempty"`
	Lobbies       []lobbySummary `json:"lobbies,omitempty"     cbor:"lobbies,omitempty"`
	Tick          int64          `json:"tick"                  cbor:"tick"`
	TickRate      int            `json:"tickRate,omitempty"    cbor:"tickRate,omitempty"`
	MapName       string         `json:"map,omitempty"         cbor:"map,omitempty"`
	State         *game.Snapshot `json:"state,omitempty"       cbor:"state,omitempty"`
	Error         string         `json:"error,omitempty"       cbor:"error,omitempty"`
}

type compactStateMessage struct {
	Type     string       `json:"t"    cbor:"t"`
	Tick     int64        `json:"tick" cbor:"tick"`
	TickRate int          `json:"r"    cbor:"r"`
	Planets  [][3]int     `json:"p"    cbor:"p"`
	Fleets   [][9]float64 `json:"f"    cbor:"f"`
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

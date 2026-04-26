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
	JoinedLobbyID string         `json:"joinedLobbyId,omitempty"`
	LobbyStatus   string         `json:"lobbyStatus,omitempty"`
	LobbyPlayers  int            `json:"lobbyPlayers,omitempty"`
	CountdownMS   int64          `json:"countdownMs,omitempty"`
	Lobbies       []lobbySummary `json:"lobbies,omitempty"`
	Tick          int64          `json:"tick"`
	TickRate      int            `json:"tickRate,omitempty"`
	MapName       string         `json:"map,omitempty"`
	State         game.Snapshot  `json:"state,omitempty"`
	Error         string         `json:"error,omitempty"`
}

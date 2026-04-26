package server

import "github.com/yyewolf/gcg/internal/game"

type clientCommand struct {
	Type   string `json:"t"`
	Source int    `json:"src"`
	Target int    `json:"dst"`
	Pct    int    `json:"pct"`
}

type outboundMessage struct {
	Type     string        `json:"t"`
	Player   int           `json:"playerId,omitempty"`
	Tick     int64         `json:"tick,omitempty"`
	TickRate int           `json:"tickRate,omitempty"`
	MapName  string        `json:"map,omitempty"`
	State    game.Snapshot `json:"state,omitempty"`
	Error    string        `json:"error,omitempty"`
}

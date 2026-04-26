package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yyewolf/gcg/internal/game"
)

func TestOutboundMessageKeepsZeroTick(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(outboundMessage{
		Type:     "welcome",
		Player:   1,
		Tick:     0,
		TickRate: 5,
		MapName:  "sector",
		State: game.Snapshot{
			Width:   100,
			Height:  100,
			Planets: []game.Planet{},
			Fleets:  []game.Fleet{},
		},
	})
	if err != nil {
		t.Fatalf("marshal outbound message: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal outbound message: %v", err)
	}

	value, ok := decoded["tick"]
	if !ok {
		t.Fatal("expected tick field to be present when zero")
	}
	if numeric, ok := value.(float64); !ok || numeric != 0 {
		t.Fatalf("expected tick field to equal 0, got %#v", value)
	}
}

func TestLobbySummariesStaySortedByCreationOrder(t *testing.T) {
	t.Parallel()

	manager := newLobbyManager()
	defer manager.close()

	manager.mu.RLock()
	first := manager.lobbies["lobby-1"]
	manager.mu.RUnlock()
	if first == nil {
		t.Fatal("expected initial lobby-1 to exist")
	}

	manager.mu.Lock()
	ctx, cancel := context.WithCancel(manager.baseCtx)
	manager.lobbies["lobby-10"] = newLobby(manager, "lobby-10", 10, cancel)
	go manager.lobbies["lobby-10"].run(ctx)
	ctx, cancel = context.WithCancel(manager.baseCtx)
	manager.lobbies["lobby-2"] = newLobby(manager, "lobby-2", 2, cancel)
	go manager.lobbies["lobby-2"].run(ctx)
	manager.mu.Unlock()

	summaries := manager.lobbySummaries()
	if len(summaries) < 3 {
		t.Fatalf("expected at least 3 lobbies, got %d", len(summaries))
	}

	if summaries[0].ID != "lobby-1" || summaries[1].ID != "lobby-2" || summaries[2].ID != "lobby-10" {
		t.Fatalf("expected creation order [lobby-1 lobby-2 lobby-10], got [%s %s %s]", summaries[0].ID, summaries[1].ID, summaries[2].ID)
	}

	_ = first
}

package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

	manager.mu.Lock()
	_, cancel := context.WithCancel(manager.baseCtx)
	manager.lobbies["lobby-1"] = newLobby(manager, "lobby-1", 1, cancel)
	_, cancel = context.WithCancel(manager.baseCtx)
	manager.lobbies["lobby-10"] = newLobby(manager, "lobby-10", 10, cancel)
	_, cancel = context.WithCancel(manager.baseCtx)
	manager.lobbies["lobby-2"] = newLobby(manager, "lobby-2", 2, cancel)
	manager.mu.Unlock()

	summaries := manager.lobbySummaries()
	if len(summaries) < 3 {
		t.Fatalf("expected at least 3 lobbies, got %d", len(summaries))
	}

	if summaries[0].ID != "lobby-1" || summaries[1].ID != "lobby-2" || summaries[2].ID != "lobby-10" {
		t.Fatalf("expected creation order [lobby-1 lobby-2 lobby-10], got [%s %s %s]", summaries[0].ID, summaries[1].ID, summaries[2].ID)
	}
}

func TestCleanupLobbyWaitsForIdleTimeout(t *testing.T) {
	t.Parallel()

	manager := newLobbyManager()
	defer manager.close()

	_, cancel := context.WithCancel(manager.baseCtx)
	defer cancel()
	lobby := newLobby(manager, "lobby-1", 1, cancel)
	manager.mu.Lock()
	manager.lobbies[lobby.id] = lobby
	manager.mu.Unlock()

	manager.cleanupLobby(lobby)
	manager.mu.RLock()
	_, ok := manager.lobbies["lobby-1"]
	manager.mu.RUnlock()
	if !ok {
		t.Fatal("expected empty lobby to survive before idle timeout")
	}

	lobby.mu.Lock()
	lobby.emptySince = time.Now().Add(-lobbyIdleCleanupDuration)
	lobby.mu.Unlock()

	manager.cleanupLobby(lobby)
	manager.mu.RLock()
	_, ok = manager.lobbies["lobby-1"]
	manager.mu.RUnlock()
	if ok {
		t.Fatal("expected idle empty lobby to be removed after timeout")
	}
}

func TestLobbyCountdownNeedsTwoPlayers(t *testing.T) {
	t.Parallel()

	manager := newLobbyManager()
	defer manager.close()

	ctx, cancel := context.WithCancel(manager.baseCtx)
	defer cancel()
	lobby := newLobby(manager, "countdown-test", 99, cancel)
	manager.mu.Lock()
	manager.lobbies[lobby.id] = lobby
	manager.mu.Unlock()
	go lobby.run(ctx)

	client := &client{manager: manager, send: make(chan []byte, 4)}
	if err := lobby.addClient(client); err != nil {
		t.Fatalf("add client: %v", err)
	}

	lobby.mu.Lock()
	lobby.countdownEndsAt = time.Time{}
	lobby.mu.Unlock()
	lobby.step()

	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	if !lobby.countdownEndsAt.IsZero() {
		t.Fatal("expected countdown to stay idle with only one player")
	}
}

func TestFinishGameRemovesPlayersFromLobby(t *testing.T) {
	t.Parallel()

	manager := newLobbyManager()
	defer manager.close()

	_, cancel := context.WithCancel(manager.baseCtx)
	defer cancel()
	lobby := newLobby(manager, "finish-test", 1, cancel)
	manager.mu.Lock()
	manager.clients = map[*client]struct{}{}
	manager.lobbies[lobby.id] = lobby
	manager.mu.Unlock()

	first := &client{manager: manager, send: make(chan []byte, 2)}
	second := &client{manager: manager, send: make(chan []byte, 2)}
	manager.mu.Lock()
	manager.clients[first] = struct{}{}
	manager.clients[second] = struct{}{}
	manager.mu.Unlock()
	if err := lobby.addClient(first); err != nil {
		t.Fatalf("add first client: %v", err)
	}
	if err := lobby.addClient(second); err != nil {
		t.Fatalf("add second client: %v", err)
	}
	first.setLobby(lobby)
	second.setLobby(lobby)

	lobby.finishGame(1)

	if first.currentLobby() != nil || second.currentLobby() != nil {
		t.Fatal("expected clients to leave the finished lobby")
	}
	if !lobby.isEmpty() {
		t.Fatal("expected finished lobby to have no remaining players")
	}
	if first.playerIDValue() != 0 || second.playerIDValue() != 0 {
		t.Fatal("expected finished clients to lose their assigned player ids")
	}
	select {
	case payload := <-first.send:
		if string(payload) == "" {
			t.Fatal("expected first client to receive a gameover payload")
		}
	default:
		t.Fatal("expected first client to receive a gameover payload")
	}
}

func TestPlayJoinsAvailableLobby(t *testing.T) {
	t.Parallel()

	manager := newLobbyManager()
	defer manager.close()

	player := &client{manager: manager, send: make(chan []byte, 4)}
	manager.mu.Lock()
	manager.clients[player] = struct{}{}
	manager.ensureOpenLobbyLocked()
	manager.mu.Unlock()

	if err := manager.play(player); err != nil {
		t.Fatalf("play: %v", err)
	}

	current := player.currentLobby()
	if current == nil {
		t.Fatal("expected play to join a lobby")
	}

	joinedID, status, players, _, playing := current.clientLobbyState()
	if joinedID == "" {
		t.Fatal("expected joined lobby id to be set")
	}
	if status != "waiting" && status != "countdown" {
		t.Fatalf("expected waiting or countdown lobby status, got %q", status)
	}
	if players != 1 {
		t.Fatalf("expected lobby to contain 1 player, got %d", players)
	}
	if playing {
		t.Fatal("expected quick-play target lobby not to be in-game")
	}
	select {
	case payload := <-player.send:
		if string(payload) == "" {
			t.Fatal("expected play to enqueue a lobby update")
		}
	default:
		t.Fatal("expected play to enqueue a lobby update")
	}

	if err := manager.play(player); err != nil {
		t.Fatalf("play same lobby: %v", err)
	}
	if player.currentLobby() != current {
		t.Fatal("expected play to keep the player in the same available lobby")
	}
}

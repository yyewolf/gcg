package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
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
		State: &game.Snapshot{
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

	state := current.clientLobbyState()
	if state.id == "" {
		t.Fatal("expected joined lobby id to be set")
	}
	if state.status != "waiting" && state.status != "countdown" {
		t.Fatalf("expected waiting or countdown lobby status, got %q", state.status)
	}
	if state.playerCount != 1 {
		t.Fatalf("expected lobby to contain 1 player, got %d", state.playerCount)
	}
	if state.playing {
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

func TestBinaryStateFrame(t *testing.T) {
	t.Parallel()

	snapshot := game.Snapshot{
		Tick:     7,
		TickRate: 30,
		Width:    200,
		Height:   100,
		Planets: []game.Planet{
			{ID: 1, Owner: 2, Ships: 18, Radius: 40, X: 10, Y: 20},
		},
		Fleets: []game.Fleet{
			{ID: 4, Owner: 2, SourceID: 1, TargetID: 3, Ships: 5, X: 11.5, Y: 22.5, VX: 3.5, VY: -1.5},
		},
		PlayerColors: []game.PlayerColor{{PlayerID: 2, Color: 0xff728c}},
	}

	frame := encodeBinaryState(snapshot, nil)

	// Header
	if frame[0] != binaryStateMagic {
		t.Fatalf("magic byte: got 0x%02x, want 0x%02x", frame[0], binaryStateMagic)
	}
	if got := int64(binary.LittleEndian.Uint64(frame[1:])); got != 7 {
		t.Fatalf("tick: got %d, want 7", got)
	}
	if frame[9] != 30 {
		t.Fatalf("tickRate: got %d, want 30", frame[9])
	}
	if got := binary.LittleEndian.Uint16(frame[10:]); got != 1 {
		t.Fatalf("nPlanets: got %d, want 1", got)
	}

	// Planet 0 at offset 12
	if got := binary.LittleEndian.Uint16(frame[12:]); got != 1 {
		t.Fatalf("planet.ID: got %d, want 1", got)
	}
	if frame[14] != 2 {
		t.Fatalf("planet.Owner: got %d, want 2", frame[14])
	}
	if got := int32(binary.LittleEndian.Uint32(frame[15:])); got != 18 {
		t.Fatalf("planet.Ships: got %d, want 18", got)
	}

	// nFleets at offset 12 + 7 = 19
	if got := binary.LittleEndian.Uint16(frame[19:]); got != 1 {
		t.Fatalf("nFleets: got %d, want 1", got)
	}

	// Fleet 0 at offset 21
	const fleetOff = 21
	if got := binary.LittleEndian.Uint32(frame[fleetOff:]); got != 4 {
		t.Fatalf("fleet.ID: got %d, want 4", got)
	}
	if frame[fleetOff+4] != 2 {
		t.Fatalf("fleet.Owner: got %d, want 2", frame[fleetOff+4])
	}
	if got := binary.LittleEndian.Uint16(frame[fleetOff+5:]); got != 1 {
		t.Fatalf("fleet.SourceID: got %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint16(frame[fleetOff+7:]); got != 3 {
		t.Fatalf("fleet.TargetID: got %d, want 3", got)
	}
	if got := binary.LittleEndian.Uint16(frame[fleetOff+9:]); got != 5 {
		t.Fatalf("fleet.Ships: got %d, want 5", got)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(frame[fleetOff+11:])); got != 11.5 {
		t.Fatalf("fleet.X: got %v, want 11.5", got)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(frame[fleetOff+15:])); got != 22.5 {
		t.Fatalf("fleet.Y: got %v, want 22.5", got)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(frame[fleetOff+19:])); got != 3.5 {
		t.Fatalf("fleet.VX: got %v, want 3.5", got)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(frame[fleetOff+23:])); got != -1.5 {
		t.Fatalf("fleet.VY: got %v, want -1.5", got)
	}

	// Static fields (width, height, playerColors) must NOT appear — the frame
	// is fixed-size binary, so there is nothing to check here beyond size.
	expectedSize := 12 + 1*7 + 2 + 1*27
	if len(frame) != expectedSize {
		t.Fatalf("frame size: got %d, want %d", len(frame), expectedSize)
	}
}

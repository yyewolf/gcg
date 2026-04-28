package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/yyewolf/gcg/internal/game"
)

const (
	lobbyCountdownDuration   = 10 * time.Second
	lobbyWaitingPollInterval = 250 * time.Millisecond
	lobbyIdleCleanupDuration = 30 * time.Second
	minimumLobbyStartPlayers = 2
)

var errLobbyFull = errors.New("lobby is full")
var errLobbyPlaying = errors.New("match already started")

type lobby struct {
	manager         *lobbyManager
	id              string
	order           int
	cancel          context.CancelFunc
	mu              sync.RWMutex
	clients         map[*client]struct{}
	clientOrder     []*client
	engine          *game.Engine
	countdownEndsAt time.Time
	emptySince      time.Time
}

func newLobby(manager *lobbyManager, id string, order int, cancel context.CancelFunc) *lobby {
	return &lobby{
		manager:    manager,
		id:         id,
		order:      order,
		cancel:     cancel,
		clients:    make(map[*client]struct{}),
		emptySince: time.Now(),
	}
}

func (lobby *lobby) run(ctx context.Context) {
	for {
		interval := lobby.nextInterval()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			lobby.step()
		}
	}
}

func (lobby *lobby) step() {
	lobby.mu.Lock()
	engine := lobby.engine
	if engine != nil {
		state := engine.Advance()
		winnerID, hasWinner := engine.Winner()
		lobby.mu.Unlock()
		lobby.broadcastState(state.Tick)
		if hasWinner {
			lobby.finishGame(winnerID)
		}
		return
	}

	now := time.Now()
	shouldBroadcastLobby := false
	started := false
	if len(lobby.clients) >= minimumLobbyStartPlayers {
		if lobby.countdownEndsAt.IsZero() {
			lobby.countdownEndsAt = now.Add(lobbyCountdownDuration)
			shouldBroadcastLobby = true
		}
		if !now.Before(lobby.countdownEndsAt) {
			lobby.startGameLocked()
			started = true
		} else {
			shouldBroadcastLobby = true
		}
	} else if !lobby.countdownEndsAt.IsZero() {
		lobby.countdownEndsAt = time.Time{}
		shouldBroadcastLobby = true
	}
	lobby.mu.Unlock()

	if started {
		lobby.broadcastGameStart()
		lobby.manager.mu.Lock()
		lobby.manager.ensureOpenLobbyLocked()
		lobby.manager.mu.Unlock()
		lobby.manager.broadcastLobbyStates()
		return
	}
	if shouldBroadcastLobby {
		lobby.manager.broadcastLobbyStates()
	}
}

func (lobby *lobby) nextInterval() time.Duration {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()

	if lobby.engine != nil {
		return time.Second / time.Duration(lobby.engine.TickRate())
	}
	if !lobby.countdownEndsAt.IsZero() {
		remaining := time.Until(lobby.countdownEndsAt)
		if remaining <= 0 {
			return 0
		}
		if remaining < lobbyWaitingPollInterval {
			return remaining
		}
	}

	return lobbyWaitingPollInterval
}

func (lobby *lobby) canJoin() error {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	if lobby.engine != nil {
		return errLobbyPlaying
	}
	if len(lobby.clients) >= maxLobbyPlayers {
		return errLobbyFull
	}

	return nil
}

func (lobby *lobby) addClient(client *client) error {
	lobby.mu.Lock()
	defer lobby.mu.Unlock()
	if lobby.engine != nil {
		return errLobbyPlaying
	}
	if len(lobby.clients) >= maxLobbyPlayers {
		return errLobbyFull
	}
	if _, ok := lobby.clients[client]; ok {
		return nil
	}

	lobby.clients[client] = struct{}{}
	lobby.clientOrder = append(lobby.clientOrder, client)
	lobby.emptySince = time.Time{}
	if len(lobby.clients) >= minimumLobbyStartPlayers && lobby.countdownEndsAt.IsZero() {
		lobby.countdownEndsAt = time.Now().Add(lobbyCountdownDuration)
	}
	return nil
}

func (lobby *lobby) removeClient(client *client) {
	lobby.mu.Lock()
	defer lobby.mu.Unlock()
	if _, ok := lobby.clients[client]; !ok {
		return
	}

	delete(lobby.clients, client)
	for index, current := range lobby.clientOrder {
		if current == client {
			lobby.clientOrder = append(lobby.clientOrder[:index], lobby.clientOrder[index+1:]...)
			break
		}
	}
	if len(lobby.clients) == 0 {
		lobby.emptySince = time.Now()
	}
	if lobby.engine == nil && len(lobby.clients) < minimumLobbyStartPlayers {
		lobby.countdownEndsAt = time.Time{}
	}
}

func (lobby *lobby) startGameLocked() {
	players := make([]*client, 0, len(lobby.clientOrder))
	for _, client := range lobby.clientOrder {
		if _, ok := lobby.clients[client]; ok {
			players = append(players, client)
		}
	}

	config := game.DefaultMapConfig()
	config.PlayerCount = len(players)
	lobby.engine = game.NewEngineWithConfig(config)
	lobby.countdownEndsAt = time.Time{}
	for index, client := range players {
		client.setPlayerID(index + 1)
	}
}

func (lobby *lobby) finishGame(winnerID int) {
	lobby.mu.Lock()
	clients := make([]*client, 0, len(lobby.clientOrder))
	for _, client := range lobby.clientOrder {
		if _, ok := lobby.clients[client]; ok {
			clients = append(clients, client)
		}
	}
	lobby.clients = make(map[*client]struct{})
	lobby.clientOrder = nil
	lobby.engine = nil
	lobby.countdownEndsAt = time.Time{}
	lobby.emptySince = time.Now()
	lobby.mu.Unlock()

	for _, client := range clients {
		client.setLobby(nil)
		client.setPlayerID(0)
		client.sendJSON(outboundMessage{Type: "gameover", Winner: winnerID})
	}

	lobby.manager.mu.Lock()
	lobby.manager.ensureOpenLobbyLocked()
	lobby.manager.mu.Unlock()
	lobby.manager.broadcastLobbyStates()
}

func (lobby *lobby) engineInstance() *game.Engine {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()

	return lobby.engine
}

func (lobby *lobby) broadcastGameStart() {
	clients := lobby.snapshotClients()
	engine := lobby.engineInstance()
	if engine == nil {
		return
	}

	for _, client := range clients {
		snap := engine.SnapshotForPlayer(client.playerIDValue())
		client.sendJSON(outboundMessage{
			Type:     "welcome",
			Player:   client.playerIDValue(),
			Tick:     engine.Tick(),
			TickRate: engine.TickRate(),
			MapName:  engine.MapName(),
			State:    &snap,
		})
	}
}

func (lobby *lobby) broadcastState(_ int64) {
	clients := lobby.snapshotClients()
	engine := lobby.engineInstance()
	if engine == nil {
		return
	}

	for _, client := range clients {
		snapshot := engine.SnapshotForPlayer(client.playerIDValue())
		client.sendJSON(newCompactStateMessage(snapshot))
	}
}

func (lobby *lobby) snapshotClients() []*client {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()

	clients := make([]*client, 0, len(lobby.clientOrder))
	for _, client := range lobby.clientOrder {
		if _, ok := lobby.clients[client]; ok {
			clients = append(clients, client)
		}
	}

	return clients
}

func (lobby *lobby) clientLobbyState() (string, string, int, int64, bool) {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	return lobby.id, lobby.statusLocked(), len(lobby.clients), lobby.countdownMSLocked(time.Now()), lobby.engine != nil
}

func (lobby *lobby) summary() lobbySummary {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()

	return lobbySummary{
		ID:          lobby.id,
		Players:     len(lobby.clients),
		MaxPlayers:  maxLobbyPlayers,
		Status:      lobby.statusLocked(),
		CountdownMS: lobby.countdownMSLocked(time.Now()),
	}
}

func (lobby *lobby) statusLocked() string {
	if lobby.engine != nil {
		return "playing"
	}
	if !lobby.countdownEndsAt.IsZero() {
		return "countdown"
	}
	return "waiting"
}

func (lobby *lobby) countdownMSLocked(now time.Time) int64 {
	if lobby.countdownEndsAt.IsZero() {
		return 0
	}
	remaining := lobby.countdownEndsAt.Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining.Milliseconds()
}

func (lobby *lobby) isWaitingOpen() bool {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	return lobby.engine == nil && lobby.countdownEndsAt.IsZero() && len(lobby.clients) < maxLobbyPlayers
}

func (lobby *lobby) isPlaying() bool {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	return lobby.engine != nil
}

func (lobby *lobby) isEmpty() bool {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	return len(lobby.clients) == 0
}

func (lobby *lobby) isExpired(now time.Time) bool {
	lobby.mu.RLock()
	defer lobby.mu.RUnlock()
	if len(lobby.clients) != 0 || lobby.emptySince.IsZero() {
		return false
	}

	return now.Sub(lobby.emptySince) >= lobbyIdleCleanupDuration
}

func (lobby *lobby) stop() {
	lobby.cancel()
}

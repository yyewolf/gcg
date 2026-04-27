package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const maxLobbyPlayers = 10

var errUnknownLobby = errors.New("lobby not found")

type lobbyManager struct {
	baseCtx   context.Context
	cancel    context.CancelFunc
	upgrader  websocket.Upgrader
	mu        sync.RWMutex
	clients   map[*client]struct{}
	lobbies   map[string]*lobby
	nextLobby int
	closed    chan struct{}
	closeOnce sync.Once
}

func newLobbyManager() *lobbyManager {
	baseCtx, cancel := context.WithCancel(context.Background())
	manager := &lobbyManager{
		baseCtx: baseCtx,
		cancel:  cancel,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		clients:   make(map[*client]struct{}),
		lobbies:   make(map[string]*lobby),
		nextLobby: 1,
		closed:    make(chan struct{}),
	}

	manager.mu.Lock()
	manager.ensureOpenLobbyLocked()
	manager.mu.Unlock()

	return manager
}

func (manager *lobbyManager) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			manager.close()
			return
		case now := <-ticker.C:
			manager.reapIdleLobbies(now)
		}
	}
}

func (manager *lobbyManager) handleWS(writer http.ResponseWriter, request *http.Request) {
	conn, err := manager.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	client := &client{
		manager: manager,
		conn:    conn,
		send:    make(chan []byte, 16),
	}

	manager.mu.Lock()
	manager.clients[client] = struct{}{}
	createdLobby := manager.ensureOpenLobbyLocked()
	manager.mu.Unlock()

	go client.writeLoop()
	manager.sendLobbyState(client)
	if createdLobby {
		manager.broadcastLobbyStates()
	}
	go client.readLoop()
}

func (manager *lobbyManager) play(client *client) error {
	manager.mu.RLock()
	var target *lobby
	for _, lobby := range manager.lobbies {
		if !lobby.isPlaying() {
			target = lobby
			break
		}
	}
	manager.mu.RUnlock()
	if target == nil {
		manager.mu.Lock()
		manager.ensureOpenLobbyLocked()
		manager.mu.Unlock()
		manager.mu.RLock()
		for _, lobby := range manager.lobbies {
			if !lobby.isPlaying() {
				target = lobby
				break
			}
		}
		manager.mu.RUnlock()
	}
	if target == nil {
		return errUnknownLobby
	}

	return manager.joinLobby(client, target.id)
}

func (manager *lobbyManager) joinLobby(client *client, lobbyID string) error {
	manager.mu.RLock()
	target := manager.lobbies[lobbyID]
	manager.mu.RUnlock()
	if target == nil {
		return errUnknownLobby
	}
	if err := target.canJoin(); err != nil {
		return err
	}

	previous := client.currentLobby()
	if previous == target {
		manager.sendLobbyState(client)
		return nil
	}

	if previous != nil {
		previous.removeClient(client)
		client.setLobby(nil)
	}

	if err := target.addClient(client); err != nil {
		if previous != nil {
			_ = previous.addClient(client)
			client.setLobby(previous)
		}
		return err
	}

	client.setLobby(target)
	manager.cleanupLobby(previous)
	manager.mu.Lock()
	manager.ensureOpenLobbyLocked()
	manager.mu.Unlock()
	manager.broadcastLobbyStates()
	return nil
}

func (manager *lobbyManager) unregister(client *client) {
	manager.mu.Lock()
	if _, ok := manager.clients[client]; !ok {
		manager.mu.Unlock()
		return
	}
	delete(manager.clients, client)
	manager.mu.Unlock()

	current := client.currentLobby()
	if current != nil {
		current.removeClient(client)
		client.setLobby(nil)
		manager.cleanupLobby(current)
	}

	if client.closeSend() {
		manager.broadcastLobbyStates()
	}
}

func (manager *lobbyManager) close() {
	manager.closeOnce.Do(func() {
		manager.cancel()
		close(manager.closed)

		manager.mu.Lock()
		clients := make([]*client, 0, len(manager.clients))
		for client := range manager.clients {
			clients = append(clients, client)
		}
		lobbies := make([]*lobby, 0, len(manager.lobbies))
		for _, lobby := range manager.lobbies {
			lobbies = append(lobbies, lobby)
		}
		manager.clients = make(map[*client]struct{})
		manager.lobbies = make(map[string]*lobby)
		manager.mu.Unlock()

		for _, lobby := range lobbies {
			lobby.stop()
		}
		for _, client := range clients {
			client.closeSend()
			if client.conn != nil {
				_ = client.conn.Close()
			}
		}
	})
}

func (manager *lobbyManager) cleanupLobby(target *lobby) {
	if target == nil {
		return
	}

	now := time.Now()
	var toStop *lobby
	manager.mu.Lock()
	if current := manager.lobbies[target.id]; current == target && target.isExpired(now) {
		delete(manager.lobbies, target.id)
		toStop = target
	}
	createdLobby := manager.ensureOpenLobbyLocked()
	manager.mu.Unlock()

	if toStop != nil {
		toStop.stop()
	}
	if toStop != nil || createdLobby {
		manager.broadcastLobbyStates()
	}
}

func (manager *lobbyManager) ensureOpenLobbyLocked() bool {
	if len(manager.clients) == 0 {
		return false
	}

	for _, lobby := range manager.lobbies {
		if lobby.isWaitingOpen() {
			return false
		}
	}

	lobbyID := fmt.Sprintf("lobby-%d", manager.nextLobby)
	order := manager.nextLobby
	manager.nextLobby++
	ctx, cancel := context.WithCancel(manager.baseCtx)
	entry := newLobby(manager, lobbyID, order, cancel)
	manager.lobbies[lobbyID] = entry
	go entry.run(ctx)
	return true
}

func (manager *lobbyManager) broadcastLobbyStates() {
	manager.mu.RLock()
	clients := make([]*client, 0, len(manager.clients))
	for client := range manager.clients {
		clients = append(clients, client)
	}
	manager.mu.RUnlock()

	summaries := manager.lobbySummaries()
	for _, client := range clients {
		lobby := client.currentLobby()
		if lobby != nil && lobby.isPlaying() {
			continue
		}
		manager.sendLobbyStateWithSummaries(client, summaries)
	}
}

func (manager *lobbyManager) sendLobbyState(client *client) {
	manager.sendLobbyStateWithSummaries(client, manager.lobbySummaries())
}

func (manager *lobbyManager) sendLobbyStateWithSummaries(client *client, summaries []lobbySummary) {
	lobby := client.currentLobby()
	payload := outboundMessage{Type: "lobby", Lobbies: summaries}
	if lobby != nil {
		joinedID, status, players, countdownMS, playing := lobby.clientLobbyState()
		if !playing {
			payload.JoinedLobbyID = joinedID
			payload.LobbyStatus = status
			payload.LobbyPlayers = players
			payload.CountdownMS = countdownMS
		}
	}
	client.sendJSON(payload)
}

func (manager *lobbyManager) lobbySummaries() []lobbySummary {
	manager.mu.RLock()
	lobbies := make([]*lobby, 0, len(manager.lobbies))
	for _, lobby := range manager.lobbies {
		lobbies = append(lobbies, lobby)
	}
	manager.mu.RUnlock()
	sort.Slice(lobbies, func(i, j int) bool {
		return lobbies[i].order < lobbies[j].order
	})

	summaries := make([]lobbySummary, 0, len(lobbies))
	for _, lobby := range lobbies {
		summaries = append(summaries, lobby.summary())
	}

	return summaries
}

func (manager *lobbyManager) reapIdleLobbies(now time.Time) {
	manager.mu.Lock()
	toStop := make([]*lobby, 0)
	for lobbyID, lobby := range manager.lobbies {
		if !lobby.isExpired(now) {
			continue
		}

		delete(manager.lobbies, lobbyID)
		toStop = append(toStop, lobby)
	}
	manager.ensureOpenLobbyLocked()
	manager.mu.Unlock()

	for _, lobby := range toStop {
		lobby.stop()
	}
	if len(toStop) > 0 {
		manager.broadcastLobbyStates()
	}
}

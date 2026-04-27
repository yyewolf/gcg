package server

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	manager  *lobbyManager
	mu       sync.RWMutex
	lobby    *lobby
	conn     *websocket.Conn
	playerID int
	send     chan []byte
	closed   bool
}

func (client *client) readLoop() {
	defer func() {
		client.manager.unregister(client)
		_ = client.conn.Close()
	}()

	client.conn.SetReadLimit(1024)

	for {
		var command clientCommand
		if err := client.conn.ReadJSON(&command); err != nil {
			return
		}

		switch command.Type {
		case "play":
			if err := client.manager.play(client); err != nil {
				client.sendJSON(outboundMessage{Type: "error", Error: err.Error()})
			}
			continue
		case "join":
			if err := client.manager.joinLobby(client, command.Lobby); err != nil {
				client.sendJSON(outboundMessage{Type: "error", Error: err.Error()})
			}
			continue
		case "send":
			lobby := client.currentLobby()
			if lobby == nil {
				client.sendJSON(outboundMessage{Type: "error", Error: "join a lobby first"})
				continue
			}

			engine := lobby.engineInstance()
			if engine == nil {
				client.sendJSON(outboundMessage{Type: "error", Error: "match has not started yet"})
				continue
			}

			if _, err := engine.SendFleet(client.playerIDValue(), command.Source, command.Target, command.Pct); err != nil {
				client.sendJSON(outboundMessage{Type: "error", Error: err.Error()})
				continue
			}

			lobby.broadcastState(engine.Tick())
		default:
			client.sendJSON(outboundMessage{Type: "error", Error: "unknown command"})
		}
	}
}

func (client *client) writeLoop() {
	pingTicker := time.NewTicker(20 * time.Second)
	defer func() {
		pingTicker.Stop()
		_ = client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server stopping"))
				return
			}

			client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-pingTicker.C:
			client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-client.manager.closed:
			return
		}
	}
}

func (client *client) sendJSON(payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal client payload failed: %v", err)
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return
	}

	select {
	case client.send <- encoded:
	default:
		go client.manager.unregister(client)
		return
	}
}

func (client *client) currentLobby() *lobby {
	client.mu.RLock()
	defer client.mu.RUnlock()

	return client.lobby
}

func (client *client) setLobby(nextLobby *lobby) {
	client.mu.Lock()
	client.lobby = nextLobby
	if nextLobby == nil {
		client.playerID = 0
	}
	client.mu.Unlock()
}

func (client *client) setPlayerID(playerID int) {
	client.mu.Lock()
	client.playerID = playerID
	client.mu.Unlock()
}

func (client *client) playerIDValue() int {
	client.mu.RLock()
	defer client.mu.RUnlock()

	return client.playerID
}

func (client *client) closeSend() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed {
		return false
	}

	client.closed = true
	close(client.send)
	return true
}

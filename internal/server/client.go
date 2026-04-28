package server

import (
	"log/slog"
	"sync"
	"time"

	cbor "github.com/fxamacker/cbor/v2"
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
		_, msgBytes, err := client.conn.ReadMessage()
		if err != nil {
			return
		}
		var command clientCommand
		if err := cbor.Unmarshal(msgBytes, &command); err != nil {
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

			lobby.nudge()
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

			if err := client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if err := client.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := client.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-client.manager.closed:
			return
		}
	}
}

func (client *client) sendJSON(payload any) {
	encoded, err := cbor.Marshal(payload)
	if err != nil {
		slog.Error("marshal client payload failed", "err", err)
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

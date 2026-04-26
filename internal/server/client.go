package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	hub      *hub
	conn     *websocket.Conn
	playerID int
	send     chan []byte
}

func (client *client) readLoop() {
	defer func() {
		client.hub.unregister(client)
		_ = client.conn.Close()
	}()

	client.conn.SetReadLimit(1024)

	for {
		var command clientCommand
		if err := client.conn.ReadJSON(&command); err != nil {
			return
		}

		switch command.Type {
		case "send":
			if _, err := client.hub.engine.SendFleet(client.playerID, command.Source, command.Target, command.Pct); err != nil {
				client.sendJSON(outboundMessage{Type: "error", Error: err.Error()})
				continue
			}

			client.hub.broadcastState(client.hub.engine.Tick())
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
		case <-client.hub.closed:
			return
		}
	}
}

func (client *client) sendJSON(payload outboundMessage) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal client payload failed: %v", err)
		return
	}

	client.hub.mu.RLock()
	_, ok := client.hub.clients[client]
	if !ok {
		client.hub.mu.RUnlock()
		return
	}

	select {
	case client.send <- encoded:
	default:
		client.hub.mu.RUnlock()
		go client.hub.unregister(client)
		return
	}
	client.hub.mu.RUnlock()
}

package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yyewolf/gcg/internal/game"
)

type hub struct {
	engine   *game.Engine
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	clients  map[*client]struct{}
	nextID   int
	closed   chan struct{}
}

func newHub(engine *game.Engine) *hub {
	return &hub{
		engine: engine,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		clients: make(map[*client]struct{}),
		nextID:  1,
		closed:  make(chan struct{}),
	}
}

func (hub *hub) run(ctx context.Context) {
	for {
		interval := time.Second / time.Duration(hub.engine.TickRate())
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			close(hub.closed)
			hub.closeAll()
			return
		case <-timer.C:
			state := hub.engine.Advance()
			hub.broadcastState(state.Tick)
		}
	}
}

func (hub *hub) handleWS(writer http.ResponseWriter, request *http.Request) {
	conn, err := hub.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	client := &client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 16),
	}

	hub.mu.Lock()
	client.playerID = hub.nextID
	hub.nextID++
	hub.clients[client] = struct{}{}
	hub.mu.Unlock()

	go client.writeLoop()
	client.sendJSON(outboundMessage{
		Type:     "welcome",
		Player:   client.playerID,
		Tick:     hub.engine.Tick(),
		TickRate: hub.engine.TickRate(),
		MapName:  hub.engine.MapName(),
		State:    hub.engine.SnapshotForPlayer(client.playerID),
	})
	go client.readLoop()
}

func (hub *hub) unregister(target *client) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if _, ok := hub.clients[target]; !ok {
		return
	}

	delete(hub.clients, target)
	close(target.send)
}

func (hub *hub) closeAll() {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for client := range hub.clients {
		close(client.send)
		_ = client.conn.Close()
		delete(hub.clients, client)
	}
}

func (hub *hub) broadcastJSON(payload outboundMessage) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal broadcast failed: %v", err)
		return
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	for client := range hub.clients {
		select {
		case client.send <- encoded:
		default:
			go hub.unregister(client)
		}
	}
}

func (hub *hub) broadcastState(tick int64) {
	hub.mu.RLock()
	clients := make([]*client, 0, len(hub.clients))
	for client := range hub.clients {
		clients = append(clients, client)
	}
	hub.mu.RUnlock()

	for _, client := range clients {
		client.sendJSON(outboundMessage{
			Type:  "state",
			Tick:  tick,
			State: hub.engine.SnapshotForPlayer(client.playerID),
		})
	}
}

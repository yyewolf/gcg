package server

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"time"
)

type Server struct {
	manager *lobbyManager
	http    *http.Server
}

func New(address string, assets fs.FS) *Server {
	manager := newLobbyManager()
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", manager.handleWS)
	mux.Handle("/", http.FileServer(http.FS(assets)))

	return &Server{
		manager: manager,
		http: &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (server *Server) Run(ctx context.Context) error {
	go server.manager.run(ctx)

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.http.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown failed: %v", err)
		}
	}()

	log.Printf("listening on http://localhost%s", server.http.Addr)
	err := server.http.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

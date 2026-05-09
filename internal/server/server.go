package server

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
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

	staticHandler := http.FileServer(http.FS(assets))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/index.html") {
			trackPageView(r)
		}
		staticHandler.ServeHTTP(w, r)
	}))

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
			slog.Error("server shutdown failed", "err", err)
		}
	}()

	slog.Info("listening", "addr", "http://localhost"+server.http.Addr)
	err := server.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

package server

import (
	"net/http"
	"os"

	"log/slog"

	"github.com/yznts/umago"
)

const (
	defaultUmagoHref    = "https://umami.yewolf.fr"
	defaultUmagoWebsite = "f9c71c5b-233e-415c-9ec5-19250ba99735"
)

var umagoConfig = umago.Configuration{
	Href:    envOr("UMAGO_HREF", defaultUmagoHref),
	Website: envOr("UMAGO_WEBSITE", defaultUmagoWebsite),
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func trackPageView(r *http.Request) {
	event := umago.NewEventFromHttpRequest(r)
	if err := umago.Send(umagoConfig, umago.NewClientFromHttpRequest(r), event); err != nil {
		slog.Error("umago page view send failed", "err", err, "url", r.URL.String())
	}
}

func trackEvent(r *http.Request, name string) {
	event := umago.NewEventFromHttpRequest(r)
	event.Name = name
	if err := umago.Send(umagoConfig, umago.NewClientFromHttpRequest(r), event); err != nil {
		slog.Error("umago event send failed", "err", err, "event", name, "url", r.URL.String())
	}
}

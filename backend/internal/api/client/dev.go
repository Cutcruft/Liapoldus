package client

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
)

// DevEventSource is the read side of the dev build hub the runtime browser
// clients connect to. The concrete *rebuilder.Hub satisfies it structurally.
type DevEventSource interface {
	Subscribe() (<-chan buildapp.DevRebuildEvent, func())
	Current(siteID string) (buildapp.DevRebuildEvent, bool)
}

// DevHandler streams current + live dev build events over /dev/build/ws. It is
// the WS-push side of the dev rebuilder (Этап 3) and the on-ramp for the boot
// hot-reload in a later stage.
type DevHandler struct {
	source DevEventSource
	logger *slog.Logger
}

func NewDevHandler(source DevEventSource, logger *slog.Logger) *DevHandler {
	return &DevHandler{source: source, logger: logger}
}

func (h *DevHandler) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	siteID := r.URL.Query().Get("siteId")

	events, unsubscribe := h.source.Subscribe()
	defer unsubscribe()

	write := func(ctx context.Context, event buildapp.DevRebuildEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return conn.Write(ctx, websocket.MessageText, data)
	}

	if last, ok := h.source.Current(siteID); ok && siteID != "" {
		if err := write(r.Context(), last); err != nil {
			return
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if siteID != "" && event.SiteID != siteID {
				continue
			}
			if err := write(r.Context(), event); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

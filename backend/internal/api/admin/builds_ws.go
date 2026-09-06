package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
)

// BuildEventSource is the read side of the build hub admin WS clients
// subscribe to (M3: live Builds page). The concrete *rebuilder.Hub satisfies
// it structurally.
type BuildEventSource interface {
	Subscribe() (<-chan buildapp.DevRebuildEvent, func())
	Current(siteID string) (buildapp.DevRebuildEvent, bool)
}

// BuildsWsHandler streams current + live build lifecycle events over
// /api/builds/ws. Browsers cannot attach Authorization headers to a
// WebSocket, so the admin token travels as ?token= and is validated before
// the upgrade (mirroring BearerAuth; empty token disables the check).
type BuildsWsHandler struct {
	source BuildEventSource
	token  string
	logger *slog.Logger
}

func NewBuildsWsHandler(source BuildEventSource, token string, logger *slog.Logger) *BuildsWsHandler {
	return &BuildsWsHandler{source: source, token: token, logger: logger}
}

func (h *BuildsWsHandler) Serve(w http.ResponseWriter, r *http.Request) {
	if h.token != "" && r.URL.Query().Get("token") != h.token {
		httpapi.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
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

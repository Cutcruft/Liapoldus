package integrationtest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/liapoldus/liapoldus/backend/internal/api/client"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/rebuilder"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDevWebSocketStreamsRebuildEvents(t *testing.T) {
	hub := rebuilder.NewHub()
	server := httptest.NewServer(http.HandlerFunc(client.NewDevHandler(hub, discardLogger()).Serve))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?siteId=site_ws"
	conn := dialWS(t, wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	event := build.DevRebuildEvent{
		SiteID: "site_ws", Environment: "development", SnapshotID: "snap",
		Status: "ready", ArtifactDir: "/build/site_ws/development/snap", UpdatedAt: time.Now().UTC(),
	}
	hub.Publish(event)

	received := readEvent(t, conn)
	if received.SiteID != event.SiteID || received.Status != "ready" || received.ArtifactDir != event.ArtifactDir {
		t.Fatalf("received %+v, want the published event", received)
	}
}

func TestDevWebSocketRelaysCurrentOnJoin(t *testing.T) {
	hub := rebuilder.NewHub()
	// Publish before anyone connects; a late joiner must get the state as the
	// first message instead of waiting for the next rebuild.
	hub.Publish(build.DevRebuildEvent{
		SiteID: "site_ws", Environment: "development", SnapshotID: "snap",
		Status: "failed", Error: "boom",
	})

	server := httptest.NewServer(http.HandlerFunc(client.NewDevHandler(hub, discardLogger()).Serve))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?siteId=site_ws"
	conn := dialWS(t, wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	received := readEvent(t, conn)
	if received.Status != "failed" || received.Error != "boom" {
		t.Fatalf("relay received %+v, want the last site event", received)
	}
}

func TestDevWebSocketFiltersBySite(t *testing.T) {
	hub := rebuilder.NewHub()
	server := httptest.NewServer(http.HandlerFunc(client.NewDevHandler(hub, discardLogger()).Serve))
	defer server.Close()

	// Client watches site_ws; events for other sites must not arrive.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?siteId=site_ws"
	conn := dialWS(t, wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	hub.Publish(build.DevRebuildEvent{SiteID: "other_site", Status: "ready"})
	hub.Publish(build.DevRebuildEvent{SiteID: "site_ws", Status: "ready"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var received build.DevRebuildEvent
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if received.SiteID != "site_ws" {
		t.Fatalf("received event for %s, want only site_ws", received.SiteID)
	}
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	return conn
}

func readEvent(t *testing.T, conn *websocket.Conn) build.DevRebuildEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("websocket read: %v", err)
	}
	var event build.DevRebuildEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	return event
}

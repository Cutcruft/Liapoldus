package unit

import (
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/rebuilder"
)

func TestHubBroadcastsToSubscribers(t *testing.T) {
	hub := rebuilder.NewHub()
	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	event := build.DevRebuildEvent{SiteID: "site_h", Environment: "development", SnapshotID: "snap_h", Status: "ready", UpdatedAt: time.Now().UTC()}
	hub.Publish(event)

	select {
	case got := <-events:
		if got.SiteID != event.SiteID || got.Status != event.Status {
			t.Fatalf("got %+v, want %+v", got, event)
		}
	case <-time.After(time.Second):
		t.Fatalf("subscriber did not receive the event")
	}
}

func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	hub := rebuilder.NewHub()
	events, unsubscribe := hub.Subscribe()
	unsubscribe()

	hub.Publish(build.DevRebuildEvent{SiteID: "site_h", Status: "ready"})

	select {
	case got, ok := <-events:
		if ok {
			t.Fatalf("unsubscribed received event %+v", got)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("channel was not closed on unsubscribe")
	}
}

func TestHubCurrentRelay(t *testing.T) {
	hub := rebuilder.NewHub()
	if _, ok := hub.Current("site_h"); ok {
		t.Fatalf("current must be empty before the first event")
	}

	event := build.DevRebuildEvent{SiteID: "site_h", Status: "failed", Error: "boom"}
	hub.Publish(event)

	current, ok := hub.Current("site_h")
	if !ok || current.Status != "failed" || current.Error != "boom" {
		t.Fatalf("current = %+v, want the published event", current)
	}
}

func TestHubSlowSubscriberDoesNotBlock(t *testing.T) {
	hub := rebuilder.NewHub()
	slow, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		hub.Publish(build.DevRebuildEvent{SiteID: "site_h", Status: "ready"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("publish blocked on a full subscriber")
	}
	// Drain the one delivered event so the deferred unsubscribe is clean.
	select {
	case <-slow:
	case <-time.After(time.Second):
	}
}

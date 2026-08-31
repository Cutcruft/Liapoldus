package grpcapi

import (
	"context"
	"log/slog"
	"net"
	"testing"

	managementv1 "github.com/liapoldus/liapoldus/backend/gen/liapoldus/management/v1"
	"github.com/liapoldus/liapoldus/backend/page"
	"github.com/liapoldus/liapoldus/backend/site"
	"github.com/liapoldus/liapoldus/backend/snapshot"
	"github.com/liapoldus/liapoldus/backend/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestManagementFlow(t *testing.T) {
	db := store.NewMemory()
	server := NewServer(site.NewService(db), page.NewService(db, db), snapshot.NewService(db, db, db), slog.Default())
	listener := bufconn.Listen(1024 * 1024)
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := managementv1.NewManagementServiceClient(conn)

	siteResponse, err := client.CreateSite(ctx, &managementv1.CreateSiteRequest{Name: "gRPC site", Slug: "grpc-site"})
	if err != nil {
		t.Fatal(err)
	}
	if siteResponse.GetSite().GetSlug() != "grpc-site" {
		t.Fatalf("site slug = %q, want grpc-site", siteResponse.GetSite().GetSlug())
	}

	pageResponse, err := client.CreatePage(ctx, &managementv1.CreatePageRequest{
		SiteId: siteResponse.GetSite().GetId(), Name: "Home", Slug: "home",
		Root: &managementv1.ComponentNode{Id: "root", Type: "Container"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pageResponse.GetPage().GetVersion() != 1 {
		t.Fatalf("page version = %d, want 1", pageResponse.GetPage().GetVersion())
	}

	updated, err := client.UpdatePageTree(ctx, &managementv1.UpdatePageTreeRequest{
		Id: pageResponse.GetPage().GetId(), Root: &managementv1.ComponentNode{Id: "root", Type: "Container"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetPage().GetVersion() != 2 {
		t.Fatalf("updated page version = %d, want 2", updated.GetPage().GetVersion())
	}

	snapshotResponse, err := client.CreateSnapshot(ctx, &managementv1.CreateSnapshotRequest{SiteId: siteResponse.GetSite().GetId(), Name: "Initial"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshotResponse.GetSnapshot().GetPages()) != 1 || snapshotResponse.GetSnapshot().GetPages()[0].GetVersion() != 2 {
		t.Fatalf("snapshot pages = %#v, want one page at version 2", snapshotResponse.GetSnapshot().GetPages())
	}
}

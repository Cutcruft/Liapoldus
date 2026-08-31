package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/liapoldus/liapoldus/backend/domain"
	"github.com/liapoldus/liapoldus/backend/esb"
	managementv1 "github.com/liapoldus/liapoldus/backend/gen/liapoldus/management/v1"
	"github.com/liapoldus/liapoldus/backend/page"
	"github.com/liapoldus/liapoldus/backend/site"
	"github.com/liapoldus/liapoldus/backend/snapshot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	managementv1.UnimplementedManagementServiceServer
	sites     *site.Service
	pages     *page.Service
	snapshots *snapshot.Service
	logger    *slog.Logger
}

func NewServer(sites *site.Service, pages *page.Service, snapshots *snapshot.Service, logger *slog.Logger, extensionRegistries ...*esb.Registry) *grpc.Server {
	server := grpc.NewServer()
	managementv1.RegisterManagementServiceServer(server, &Server{
		sites: sites, pages: pages, snapshots: snapshots, logger: logger,
	})
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	var registry *esb.Registry
	if len(extensionRegistries) > 0 {
		registry = extensionRegistries[0]
	}
	esb.RegisterGRPC(server, registry)
	return server
}

func (s *Server) CreateSite(ctx context.Context, req *managementv1.CreateSiteRequest) (*managementv1.CreateSiteResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.sites.Create(ctx, req.GetName(), req.GetSlug())
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.CreateSiteResponse{Site: siteMessage(result)}, nil
}

func (s *Server) GetSite(ctx context.Context, req *managementv1.GetSiteRequest) (*managementv1.GetSiteResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.sites.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.GetSiteResponse{Site: siteMessage(result)}, nil
}

func (s *Server) CreatePage(ctx context.Context, req *managementv1.CreatePageRequest) (*managementv1.CreatePageResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.pages.Create(ctx, req.GetSiteId(), req.GetName(), req.GetSlug(), nodeDomain(req.GetRoot()))
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.CreatePageResponse{Page: pageMessage(result)}, nil
}

func (s *Server) GetPage(ctx context.Context, req *managementv1.GetPageRequest) (*managementv1.GetPageResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.pages.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.GetPageResponse{Page: pageMessage(result)}, nil
}

func (s *Server) ListPages(ctx context.Context, req *managementv1.ListPagesRequest) (*managementv1.ListPagesResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.pages.ListBySite(ctx, req.GetSiteId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &managementv1.ListPagesResponse{Pages: make([]*managementv1.Page, 0, len(result))}
	for _, item := range result {
		response.Pages = append(response.Pages, pageMessage(item))
	}
	return response, nil
}

func (s *Server) UpdatePageTree(ctx context.Context, req *managementv1.UpdatePageTreeRequest) (*managementv1.UpdatePageTreeResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.pages.UpdateTree(ctx, req.GetId(), nodeDomain(req.GetRoot()))
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.UpdatePageTreeResponse{Page: pageMessage(result)}, nil
}

func (s *Server) ListPageVersions(ctx context.Context, req *managementv1.ListPageVersionsRequest) (*managementv1.ListPageVersionsResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.pages.Versions(ctx, req.GetPageId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &managementv1.ListPageVersionsResponse{Versions: make([]*managementv1.PageVersion, 0, len(result))}
	for _, item := range result {
		response.Versions = append(response.Versions, pageVersionMessage(item))
	}
	return response, nil
}

func (s *Server) CreateSnapshot(ctx context.Context, req *managementv1.CreateSnapshotRequest) (*managementv1.CreateSnapshotResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.snapshots.Create(ctx, req.GetSiteId(), req.GetName())
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.CreateSnapshotResponse{Snapshot: snapshotMessage(result)}, nil
}

func (s *Server) GetSnapshot(ctx context.Context, req *managementv1.GetSnapshotRequest) (*managementv1.GetSnapshotResponse, error) {
	if req == nil {
		return nil, invalidArgument("request is required")
	}
	result, err := s.snapshots.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return &managementv1.GetSnapshotResponse{Snapshot: snapshotMessage(result)}, nil
}

func nodeDomain(node *managementv1.ComponentNode) domain.ComponentNode {
	if node == nil {
		return domain.ComponentNode{}
	}
	result := domain.ComponentNode{ID: node.GetId(), Type: node.GetType()}
	if props := node.GetProps(); props != nil {
		result.Props = props.AsMap()
	}
	for _, child := range node.GetChildren() {
		result.Children = append(result.Children, nodeDomain(child))
	}
	return result
}

func nodeMessage(node domain.ComponentNode) *managementv1.ComponentNode {
	result := &managementv1.ComponentNode{Id: node.ID, Type: node.Type}
	if node.Props != nil {
		result.Props, _ = structpb.NewStruct(node.Props)
	}
	for _, child := range node.Children {
		result.Children = append(result.Children, nodeMessage(child))
	}
	return result
}

func siteMessage(site domain.Site) *managementv1.Site {
	return &managementv1.Site{Id: site.ID, Name: site.Name, Slug: site.Slug, CreatedAt: timestamppb.New(site.CreatedAt)}
}

func pageMessage(page domain.Page) *managementv1.Page {
	return &managementv1.Page{Id: page.ID, SiteId: page.SiteID, Name: page.Name, Slug: page.Slug, Root: nodeMessage(page.Root), Version: page.Version, CreatedAt: timestamppb.New(page.CreatedAt), UpdatedAt: timestamppb.New(page.UpdatedAt)}
}

func pageVersionMessage(version domain.PageVersion) *managementv1.PageVersion {
	return &managementv1.PageVersion{Id: version.ID, PageId: version.PageID, Number: version.Number, Root: nodeMessage(version.Root), CreatedAt: timestamppb.New(version.CreatedAt)}
}

func snapshotMessage(snapshot domain.Snapshot) *managementv1.Snapshot {
	result := &managementv1.Snapshot{Id: snapshot.ID, SiteId: snapshot.SiteID, Name: snapshot.Name, CreatedAt: timestamppb.New(snapshot.CreatedAt)}
	for _, page := range snapshot.Pages {
		result.Pages = append(result.Pages, &managementv1.SnapshotPage{PageId: page.PageID, VersionId: page.VersionID, Version: page.Version})
	}
	return result
}

func invalidArgument(message string) error { return status.Error(codes.InvalidArgument, message) }

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}

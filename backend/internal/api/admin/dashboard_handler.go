package admin

import (
	"net/http"
	"sort"
	"time"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

const dashboardRecentLimit = 5

// DashboardHandler aggregates high-level state for the admin Обзор page:
// site inventory (with the full git status chosen by the product owner),
// the most recent builds and snapshots, and a derived runtime status.
type DashboardHandler struct {
	sites     *site.Service
	builds    *buildapp.Service
	snapshots *snapshot.Service
	git       *gitsnapshot.Service
}

func NewDashboardHandler(sites *site.Service, builds *buildapp.Service, snapshots *snapshot.Service, git *gitsnapshot.Service) *DashboardHandler {
	return &DashboardHandler{sites: sites, builds: builds, snapshots: snapshots, git: git}
}

type dashboardSite struct {
	SiteID        string                  `json:"siteId"`
	Name          string                  `json:"name"`
	Slug          string                  `json:"slug"`
	DefaultLocale string                  `json:"defaultLocale"`
	Hosts         []string                `json:"hosts"`
	Git           *gitsnapshot.SiteStatus `json:"git,omitempty"`
}

type recentBuild struct {
	ID          string             `json:"id"`
	SiteID      string             `json:"siteId"`
	SiteName    string             `json:"siteName"`
	SnapshotID  string             `json:"snapshotId"`
	Environment string             `json:"environment"`
	Status      domain.BuildStatus `json:"status"`
	CreatedAt   time.Time          `json:"createdAt"`
}

type recentSnapshot struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"siteId"`
	SiteName  string    `json:"siteName"`
	Name      string    `json:"name"`
	GitSHA    string    `json:"gitSha,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type dashboardResponse struct {
	SiteCount       int              `json:"siteCount"`
	Sites           []dashboardSite  `json:"sites"`
	RecentBuilds    []recentBuild    `json:"recentBuilds"`
	RecentSnapshots []recentSnapshot `json:"recentSnapshots"`
	RuntimeStatus   string           `json:"runtimeStatus"`
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := dashboardResponse{
		Sites:           []dashboardSite{},
		RecentBuilds:    []recentBuild{},
		RecentSnapshots: []recentSnapshot{},
	}

	sites, err := h.sites.List(ctx)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	resp.SiteCount = len(sites)
	siteByName := make(map[string]string, len(sites))

	allBuilds := []domain.Build{}
	allSnapshots := []domain.Snapshot{}
	anyFailed, anyReady := false, false

	for _, site := range sites {
		siteByName[site.ID] = site.Name
		dashboard := dashboardSite{
			SiteID:        site.ID,
			Name:          site.Name,
			Slug:          site.Slug,
			DefaultLocale: site.DefaultLocale,
			Hosts:         site.Hosts,
		}
		if h.git != nil {
			if status, err := h.git.Status(ctx, site.ID); err == nil && status != nil {
				dashboard.Git = status
			}
		}
		resp.Sites = append(resp.Sites, dashboard)

		if builds, err := h.builds.ListBySite(ctx, site.ID); err == nil {
			allBuilds = append(allBuilds, builds...)
			for _, b := range builds {
				switch b.Status {
				case domain.BuildStatusFailed:
					anyFailed = true
				case domain.BuildStatusReady:
					anyReady = true
				}
			}
		}
		if snapshots, err := h.snapshots.ListBySite(ctx, site.ID); err == nil {
			allSnapshots = append(allSnapshots, snapshots...)
		}
	}

	sort.Slice(allBuilds, func(i, j int) bool { return allBuilds[i].CreatedAt.After(allBuilds[j].CreatedAt) })
	for i := 0; i < len(allBuilds) && i < dashboardRecentLimit; i++ {
		b := allBuilds[i]
		resp.RecentBuilds = append(resp.RecentBuilds, recentBuild{
			ID:          b.ID,
			SiteID:      b.SiteID,
			SiteName:    siteByName[b.SiteID],
			SnapshotID:  b.SnapshotID,
			Environment: b.Environment,
			Status:      b.Status,
			CreatedAt:   b.CreatedAt,
		})
	}

	sort.Slice(allSnapshots, func(i, j int) bool { return allSnapshots[i].CreatedAt.After(allSnapshots[j].CreatedAt) })
	for i := 0; i < len(allSnapshots) && i < dashboardRecentLimit; i++ {
		s := allSnapshots[i]
		resp.RecentSnapshots = append(resp.RecentSnapshots, recentSnapshot{
			ID:        s.ID,
			SiteID:    s.SiteID,
			SiteName:  siteByName[s.SiteID],
			Name:      s.Name,
			GitSHA:    s.GitSHA,
			CreatedAt: s.CreatedAt,
		})
	}

	switch {
	case anyFailed:
		resp.RuntimeStatus = "degraded"
	case anyReady:
		resp.RuntimeStatus = "ok"
	default:
		resp.RuntimeStatus = "no-builds"
	}

	httpapi.RespondJSON(w, http.StatusOK, resp)
}

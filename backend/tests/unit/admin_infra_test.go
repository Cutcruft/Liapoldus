package unit

import (
	"context"
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func r6OperationRequest() map[string]any {
	return map[string]any{
		"id":         "contact.export",
		"provider":   "liapoldus.builtin",
		"typeOp":     "query",
		"method":     "GET",
		"path":       "/lib/api/contact/export",
		"cache":      "ttl",
		"ttl":        60,
		"scope":      "public",
		"resultType": "content[]",
		"params":     map[string]any{"limit": map[string]any{"type": "number"}},
	}
}

// TestAdminOperationCRUD covers the R6 operations slice: create → list →
// update → delete, plus validation and the system read-only guard.
func TestAdminOperationCRUD(t *testing.T) {
	app, _ := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "opsite", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := admin.NewRouter(app)
	base := "/api/sites/" + site.ID + "/operations"

	create := request(t, handler, http.MethodPost, base, r6OperationRequest())
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", create.Code, create.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		System   bool   `json:"system"`
		SiteID   string `json:"siteId"`
		TypeOp   string `json:"typeOp"`
		Provider string `json:"provider"`
		TTL      int    `json:"ttl"`
	}
	decodeResponse(t, create, &created)
	if created.ID != "contact.export" || created.System || created.SiteID != site.ID ||
		created.TypeOp != "query" || created.Provider != "liapoldus.builtin" || created.TTL != 60 {
		t.Fatalf("created = %#v", created)
	}

	// Duplicate id → 409 even after the id exists in the site.
	dup := request(t, handler, http.MethodPost, base, r6OperationRequest())
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", dup.Code, dup.Body.String())
	}

	// List shows only the site operation (no system rows seeded here).
	list := request(t, handler, http.MethodGet, base, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", list.Code, list.Body.String())
	}
	var ops []struct {
		ID string `json:"id"`
	}
	decodeResponse(t, list, &ops)
	if len(ops) != 1 || ops[0].ID != "contact.export" {
		t.Fatalf("list = %#v", ops)
	}

	// Update flips the method and cache.
	update := request(t, handler, http.MethodPut, base+"/contact.export", map[string]any{
		"provider": "liapoldus.builtin",
		"typeOp":   "query",
		"method":   "POST",
		"path":     "/lib/api/contact/export",
		"cache":    "disabled",
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", update.Code, update.Body.String())
	}
	var updated struct {
		Method string `json:"method"`
		Cache  string `json:"cache"`
	}
	decodeResponse(t, update, &updated)
	if updated.Method != "POST" || updated.Cache != "disabled" {
		t.Fatalf("updated = %#v", updated)
	}

	// Get resolves the site row.
	get := request(t, handler, http.MethodGet, base+"/contact.export", nil)
	if get.Code != http.StatusOK || get.Body.String() == "" {
		t.Fatalf("get status = %d, body = %s", get.Code, get.Body.String())
	}

	// Unknown id → 404.
	missing := request(t, handler, http.MethodGet, base+"/nope", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", missing.Code)
	}

	// Delete → 204, then 404 on re-read.
	del := request(t, handler, http.MethodDelete, base+"/contact.export", nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", del.Code, del.Body.String())
	}
	after := request(t, handler, http.MethodGet, base+"/contact.export", nil)
	if after.Code != http.StatusNotFound {
		t.Fatalf("after delete status = %d", after.Code)
	}
}

// TestAdminOperationValidation rejects malformed operations and guards system
// rows from writes.
func TestAdminOperationValidation(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "opsite2", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := admin.NewRouter(app)
	base := "/api/sites/" + site.ID + "/operations"

	cases := []map[string]any{
		{},                                                                          // empty id
		{"id": "BadID", "typeOp": "query", "method": "GET", "path": "/x", "cache": "disabled"}, // bad id chars
		{"id": "op", "typeOp": "whatever", "method": "GET", "path": "/x", "cache": "disabled"}, // bad typeOp
		{"id": "op", "typeOp": "query", "method": "TRACE", "path": "/x", "cache": "disabled"},  // bad method
		{"id": "op", "typeOp": "query", "method": "GET", "path": "x", "cache": "disabled"},     // path without slash
		{"id": "op", "typeOp": "query", "method": "GET", "path": "/x", "cache": "ttl"},         // ttl cache without ttl
	}
	for i, body := range cases {
		res := request(t, handler, http.MethodPost, base, body)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("case %d status = %d, body = %s", i, res.Code, res.Body.String())
		}
	}

	// Seed a system row straight through the store — it must be read-only.
	if err := db.CreateOperation(ctx, domain.Operation{
		ID: "system.op", SiteID: "", System: true, Provider: "liapoldus.builtin",
		TypeOp: "query", Method: "GET", Path: "/lib/api/system", Cache: "immutable",
	}); err != nil {
		t.Fatal(err)
	}
	// A system id cannot be shadowed by a site id either.
	shadow := request(t, handler, http.MethodPost, base, map[string]any{
		"id": "system.op", "typeOp": "query", "method": "GET", "path": "/own", "cache": "disabled",
	})
	if shadow.Code != http.StatusConflict {
		t.Fatalf("shadow status = %d, body = %s", shadow.Code, shadow.Body.String())
	}
	// Update/delete on the system id → 400.
	sysUpdate := request(t, handler, http.MethodPut, base+"/system.op", map[string]any{
		"typeOp": "query", "method": "GET", "path": "/lib/api/system", "cache": "immutable",
	})
	if sysUpdate.Code != http.StatusBadRequest {
		t.Fatalf("system update status = %d, body = %s", sysUpdate.Code, sysUpdate.Body.String())
	}
	sysDelete := request(t, handler, http.MethodDelete, base+"/system.op", nil)
	if sysDelete.Code != http.StatusBadRequest {
		t.Fatalf("system delete status = %d, body = %s", sysDelete.Code, sysDelete.Body.String())
	}
}

// TestAdminEndpointCRUD covers endpoint CRUD and its operation reference.
func TestAdminEndpointCRUD(t *testing.T) {
	app, _ := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "epsite", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := admin.NewRouter(app)

	opsBase := "/api/sites/" + site.ID + "/operations"
	epBase := "/api/sites/" + site.ID + "/endpoints"

	// Create the referenced operation first.
	if res := request(t, handler, http.MethodPost, opsBase, map[string]any{
		"id": "booking.save", "typeOp": "mutation", "method": "POST",
		"path": "/api/booking/save", "cache": "disabled",
	}); res.Code != http.StatusCreated {
		t.Fatalf("create op status = %d, body = %s", res.Code, res.Body.String())
	}

	create := request(t, handler, http.MethodPost, epBase, map[string]any{
		"id": "booking", "method": "POST", "path": "/api/forms/booking", "operationId": "booking.save",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create endpoint status = %d, body = %s", create.Code, create.Body.String())
	}
	var created struct {
		ID          string `json:"id"`
		OperationID string `json:"operationId"`
		System      bool   `json:"system"`
	}
	decodeResponse(t, create, &created)
	if created.ID != "booking" || created.OperationID != "booking.save" || created.System {
		t.Fatalf("created = %#v", created)
	}

	// An endpoint cannot point at a missing operation.
	bad := request(t, handler, http.MethodPost, epBase, map[string]any{
		"id": "ghost", "method": "POST", "path": "/api/forms/ghost", "operationId": "no.such.op",
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("ghost operation status = %d, body = %s", bad.Code, bad.Body.String())
	}

	// List + delete.
	list := request(t, handler, http.MethodGet, epBase, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}
	var eps []struct {
		ID string `json:"id"`
	}
	decodeResponse(t, list, &eps)
	if len(eps) != 1 || eps[0].ID != "booking" {
		t.Fatalf("list = %#v", eps)
	}
	if res := request(t, handler, http.MethodDelete, epBase+"/booking", nil); res.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", res.Code, res.Body.String())
	}
}
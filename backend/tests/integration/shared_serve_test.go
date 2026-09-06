package integrationtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
)

// TestSharedBundlesServedFromArtifactRoot proves the shared bundles installed
// by shared.Install are publicly reachable through the exact handler the
// client server uses for /build/ (http.FileServer over the artifact root).
func TestSharedBundlesServedFromArtifactRoot(t *testing.T) {
	root := t.TempDir()
	if err := shared.Install(root); err != nil {
		t.Fatalf("install: %v", err)
	}
	handler := http.StripPrefix("/build/", http.FileServer(http.Dir(root)))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	for _, path := range []string{
		"/build/_shared/react/18.3.1.js",
		"/build/_shared/react-dom/18.3.1.js",
		"/build/_shared/react/jsx-runtime/18.3.1.js",
		"/build/_shared/@liapoldus/ui-runtime/0.1.0.js",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, res.StatusCode)
		}
		if got := res.Header.Get("content-type"); !strings.Contains(got, "javascript") {
			t.Fatalf("GET %s content-type = %q, want javascript", path, got)
		}
		if len(body) == 0 {
			t.Fatalf("GET %s returned an empty body", path)
		}
	}

	// Unversioned/unknown bundles are not served.
	for _, path := range []string{
		"/build/_shared/react/999.0.0.js",
		"/build/_shared/nope/0.1.0.js",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, res.StatusCode)
		}
	}
}

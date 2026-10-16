package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandlerServesIndexAndFallback(t *testing.T) {
	h := newHandler(fstest.MapFS{
		"index.html":  {Data: []byte("<div id=root></div>")},
		"assets/a.js": {Data: []byte("console.log(1)")},
	})

	for _, path := range []string{"/", "/pots/3"} {
		rec := get(h, path)
		if rec.Code != http.StatusOK || rec.Body.String() != "<div id=root></div>" {
			t.Errorf("%s: got %d %q", path, rec.Code, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("%s: Cache-Control = %q", path, cc)
		}
	}

	rec := get(h, "/assets/a.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("asset: got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q", cc)
	}

	if rec := get(h, "/missing.js"); rec.Code != http.StatusNotFound {
		t.Errorf("/missing.js: got %d, want 404", rec.Code)
	}
	if rec := get(h, "/api/v1/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("/api/v1/nope: got %d, want 404", rec.Code)
	}
}

func TestHandlerNotBuilt(t *testing.T) {
	h := newHandler(fstest.MapFS{".gitkeep": {}})
	for _, path := range []string{"/", "/pots/3"} {
		rec := get(h, path)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d, want 503", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "dashboard not built: run npm run build in web/") {
			t.Errorf("%s: body %q", path, rec.Body.String())
		}
	}
}

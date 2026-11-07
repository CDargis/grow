package oils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The fetch side of S3Archiver, without S3: a JPG served as octet-stream is
// sniffed as image/jpeg; bad links fail with a reason. The S3 put isn't
// exercised here (no client), so these call fetch directly.
func TestArchiverFetch(t *testing.T) {
	jpg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("JFIF fake image body")...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/coa.jpg":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(jpg)
		case "/gone":
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := &S3Archiver{client: srv.Client()}
	body, ct, err := a.fetch(context.Background(), srv.URL+"/coa.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/jpeg" || extensionFor(ct) != ".jpg" || len(body) != len(jpg) {
		t.Errorf("content type = %q, ext = %q, %d bytes", ct, extensionFor(ct), len(body))
	}

	if _, _, err := a.fetch(context.Background(), srv.URL+"/gone"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404 link: err = %v", err)
	}
	if _, _, err := a.fetch(context.Background(), "ftp://x/y.pdf"); err == nil {
		t.Error("non-http link should be rejected")
	}
}

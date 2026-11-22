package updater

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	if compareVersions("2.0.1", "2.0.0") != 1 {
		t.Fatal("expected 2.0.1 > 2.0.0")
	}
	if compareVersions("2.0.0", "2.0.0") != 0 {
		t.Fatal("expected 2.0.0 == 2.0.0")
	}
	if compareVersions("1.9.9", "2.0.0") != -1 {
		t.Fatal("expected 1.9.9 < 2.0.0")
	}
}

func TestDownloadUpdateValidation(t *testing.T) {
	if _, err := DownloadUpdate("http://example.com/update.zip", nil); err == nil {
		t.Fatal("expected HTTP URL to be rejected when HTTPS is required")
	}

	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer errSrv.Close()

	if _, err := downloadUpdateWithClient(errSrv.Client(), errSrv.URL+"/app.zip", nil, true); err == nil {
		t.Fatal("expected non-200 HTTP status to fail download")
	}

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("syncforge-binary-payload"))
	}))
	defer okSrv.Close()

	progress := make(chan int, 8)
	path, err := downloadUpdateWithClient(okSrv.Client(), okSrv.URL+"/app.zip", progress, true)
	if err != nil {
		t.Fatalf("expected 200 download to succeed, got %v", err)
	}
	defer os.Remove(path)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded temp file failed: %v", err)
	}
	if string(raw) != "syncforge-binary-payload" {
		t.Fatalf("unexpected downloaded payload: %q", string(raw))
	}
}

func TestValidateReleaseURL(t *testing.T) {
	if got, err := validateReleaseURL("https://github.com/nanablast/syncforge/releases/tag/v2.0.0"); err != nil || !strings.HasPrefix(got, "https://") {
		t.Fatalf("expected valid HTTPS URL, got %q err=%v", got, err)
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///etc/passwd", "not a url", "https://"} {
		if _, err := validateReleaseURL(bad); err == nil {
			t.Fatalf("expected invalid/unsafe URL %q to be rejected", bad)
		}
	}
}

func TestShellQuote(t *testing.T) {
	in := `/Applications/My App's "Dir"/SyncForge.app`
	q := shellQuote(in)
	if q != `'/Applications/My App'"'"'s "Dir"/SyncForge.app'` {
		t.Fatalf("unexpected shellQuote output: %s", q)
	}
}

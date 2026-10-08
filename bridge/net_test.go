package bridge

import (
	"net/http"
	"testing"
)

func TestRequestProxyDirectSkipsEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:9")
	t.Setenv("http_proxy", "http://127.0.0.1:9")

	req, err := http.NewRequest(http.MethodGet, "http://example.com/ip", nil)
	if err != nil {
		t.Fatal(err)
	}

	direct, err := requestProxy("direct")(req)
	if err != nil {
		t.Fatal(err)
	}
	if direct != nil {
		t.Fatalf("direct dial used a proxy: %s", direct)
	}

	fromEnv, err := requestProxy("")(req)
	if err != nil {
		t.Fatal(err)
	}
	if fromEnv == nil || fromEnv.Host != "127.0.0.1:9" {
		t.Fatalf("empty proxy = %v, want the environment proxy", fromEnv)
	}
}

func TestLookupHostLocalhost(t *testing.T) {
	got := (&App{}).LookupHost("localhost")
	if !got.Flag || got.Data == "" {
		t.Fatalf("localhost lookup = %+v", got)
	}
}

func TestLookupHostRejectsEmptyName(t *testing.T) {
	got := (&App{}).LookupHost("  ")
	if got.Flag {
		t.Fatalf("empty host was accepted: %+v", got)
	}
}

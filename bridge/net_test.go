package bridge

import "testing"

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

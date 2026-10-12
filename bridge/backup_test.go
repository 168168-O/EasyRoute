package bridge

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupRoundTripKeepsSettingsAndDropsUnknownPaths(t *testing.T) {
	root := t.TempDir()
	Env.BasePath = root
	t.Cleanup(func() { Env.BasePath = "" })

	mustWrite := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("data/user.yaml", "exitOnClose: false\n")
	mustWrite("data/subscribes.yaml", "subs: []\n")
	mustWrite("data/profiles.yaml", "profiles: []\n")
	mustWrite("data/subscribes/node.json", `[{"tag":"hk"}]`)
	mustWrite("data/user-bg-video.mp4", "video")
	mustWrite("data/secrets/token.txt", "nope")

	app := &App{}
	exported := app.ExportBackup("")
	if !exported.Flag {
		t.Fatal(exported.Data)
	}
	reader, err := zip.OpenReader(exported.Data)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	reader.Close()
	joined := strings.Join(names, "\n")
	for _, want := range []string{"data/user.yaml", "data/subscribes.yaml", "data/profiles.yaml", "data/subscribes/node.json", "data/user-bg-video.mp4"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
	if strings.Contains(joined, "secrets") {
		t.Fatalf("unexpected secret in %s", joined)
	}

	os.WriteFile(filepath.Join(root, "data", "user.yaml"), []byte("changed"), 0o644)
	restored := app.RestoreBackup(exported.Data)
	if !restored.Flag {
		t.Fatal(restored.Data)
	}
	body, err := os.ReadFile(filepath.Join(root, "data", "user.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "exitOnClose: false\n" {
		t.Fatalf("restored %q", body)
	}
}

func TestAutoBackupKeepsFive(t *testing.T) {
	root := t.TempDir()
	Env.BasePath = root
	t.Cleanup(func() { Env.BasePath = "" })
	full := filepath.Join(root, "data", "user.yaml")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "data", "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auto-20000101-000001.zip", "auto-20000101-000002.zip", "auto-20000101-000003.zip", "auto-20000101-000004.zip", "auto-20000101-000005.zip"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{}
	result := app.AutoBackup()
	if !result.Flag {
		t.Fatal(result.Data)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var autos int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "auto-") {
			autos++
		}
	}
	if autos != 5 {
		t.Fatalf("kept %d", autos)
	}
	if _, err := os.Stat(filepath.Join(dir, "auto-20000101-000001.zip")); !os.IsNotExist(err) {
		t.Fatal("oldest auto backup was kept")
	}
}

package bridge

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ListProcesses returns running programs grouped by exe name.
func (a *App) ListProcesses() FlagResult {
	procs, err := process.Processes()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	type item struct {
		Name string `json:"name"`
		Exe  string `json:"exe"`
	}
	seen := map[string]item{}
	for _, proc := range procs {
		exeName, err := proc.Name()
		if err != nil || exeName == "" {
			continue
		}
		exe := exeName
		if exePath, pathErr := proc.Exe(); pathErr == nil && exePath != "" {
			exe = filepath.Base(exePath)
		}
		key := strings.ToLower(exe)
		if _, ok := seen[key]; ok {
			continue
		}
		name := strings.TrimSuffix(exe, filepath.Ext(exe))
		seen[key] = item{Name: name, Exe: exe}
		if len(seen) >= 400 {
			break
		}
	}

	list := make([]item, 0, len(seen))
	for _, entry := range seen {
		list = append(list, entry)
	}
	payload, err := json.Marshal(list)
	if err != nil {
		return FlagResult{false, err.Error()}
	}
	return FlagResult{true, string(payload)}
}

// ListDouyinExes finds extra Douyin executables under the default install directory.
func (a *App) ListDouyinExes() FlagResult {
	dir := `C:\Program Files (x86)\ByteDance\douyin`
	names := []string{}
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".exe") {
			names = append(names, entry.Name())
		}
		return nil
	})
	payload, err := json.Marshal(names)
	if err != nil {
		return FlagResult{false, err.Error()}
	}
	return FlagResult{true, string(payload)}
}

// PickFile opens a native file dialog. pattern is a semicolon-separated list such as "*.exe".
func (a *App) PickFile(title string, pattern string) FlagResult {
	if a.Ctx == nil {
		return FlagResult{false, "window is not ready"}
	}
	if pattern == "" {
		pattern = "*.*"
	}
	path, err := runtime.OpenFileDialog(a.Ctx, runtime.OpenDialogOptions{
		Title: title,
		Filters: []runtime.FileFilter{
			{DisplayName: title, Pattern: pattern},
		},
	})
	if err != nil {
		return FlagResult{false, err.Error()}
	}
	if path == "" {
		return FlagResult{false, "cancelled"}
	}
	return FlagResult{true, path}
}

var (
	bgVideoOnce sync.Once
	bgVideoURL  string
	bgVideoErr  error
)

// BackgroundVideoURL serves the user-selected background from 127.0.0.1 so the
// webview can stream it without loading the whole file into memory.
func (a *App) BackgroundVideoURL() FlagResult {
	bgVideoOnce.Do(func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			bgVideoErr = err
			return
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {
			for _, name := range []string{"data/user-bg-video.mp4", "data/user-bg-video.webm"} {
				full := resolvePath(name)
				info, statErr := os.Stat(full)
				if statErr != nil || info.IsDir() {
					continue
				}
				w.Header().Set("Cache-Control", "no-store")
				http.ServeFile(w, r, full)
				return
			}
			http.NotFound(w, r)
		})
		server := &http.Server{Handler: mux}
		go server.Serve(listener)
		bgVideoURL = "http://" + listener.Addr().String() + "/video"
	})
	if bgVideoErr != nil {
		return FlagResult{false, bgVideoErr.Error()}
	}
	return FlagResult{true, bgVideoURL}
}

package bridge

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func backupRoot() string {
	return filepath.Clean(Env.BasePath)
}

func isBackupPath(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return false
	}
	switch rel {
	case "data/user.yaml", "data/subscribes.yaml", "data/profiles.yaml", "data/user-bg-video.mp4", "data/user-bg-video.webm":
		return true
	default:
		return strings.HasPrefix(rel, "data/subscribes/") && !strings.HasSuffix(rel, "/")
	}
}

func collectBackupFiles(root string) []string {
	var found []string
	fixed := []string{
		"data/user.yaml",
		"data/subscribes.yaml",
		"data/profiles.yaml",
		"data/user-bg-video.mp4",
		"data/user-bg-video.webm",
	}
	for _, rel := range fixed {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && !info.IsDir() {
			found = append(found, rel)
		}
	}
	subDir := filepath.Join(root, "data", "subscribes")
	entries, err := os.ReadDir(subDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			rel := "data/subscribes/" + entry.Name()
			if isBackupPath(rel) {
				found = append(found, rel)
			}
		}
	}
	return found
}

func writeBackupZip(root, dest string, files []string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	for _, rel := range files {
		if !isBackupPath(rel) {
			continue
		}
		src, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			zw.Close()
			return err
		}
		entry, err := zw.Create(rel)
		if err != nil {
			src.Close()
			zw.Close()
			return err
		}
		_, err = io.Copy(entry, src)
		src.Close()
		if err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}

func restoreBackupZip(root, src string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		rel := filepath.ToSlash(file.Name)
		if !isBackupPath(rel) || file.FileInfo().IsDir() {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func rotateAutoBackups(root string) (string, error) {
	files := collectBackupFiles(root)
	if len(files) == 0 {
		return "", fmt.Errorf("没有可以备份的数据")
	}
	dir := filepath.Join(root, "data", "backups")
	name := "auto-" + time.Now().Format("20060102-150405") + ".zip"
	dest := filepath.Join(dir, name)
	if err := writeBackupZip(root, dest, files); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dest, nil
	}
	var autos []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "auto-") && strings.HasSuffix(entry.Name(), ".zip") {
			autos = append(autos, entry.Name())
		}
	}
	sort.Strings(autos)
	for len(autos) > 5 {
		os.Remove(filepath.Join(dir, autos[0]))
		autos = autos[1:]
	}
	return dest, nil
}

func (a *App) ExportBackup(dest string) FlagResult {
	root := backupRoot()
	files := collectBackupFiles(root)
	if len(files) == 0 {
		return FlagResult{Flag: false, Data: "没有可以备份的数据"}
	}
	if strings.TrimSpace(dest) == "" {
		dest = filepath.Join(root, "data", "backups", "backup-"+time.Now().Format("20060102-150405")+".zip")
	} else if !filepath.IsAbs(dest) {
		dest = filepath.Join(root, dest)
	}
	if err := writeBackupZip(root, dest, files); err != nil {
		return FlagResult{Flag: false, Data: err.Error()}
	}
	return FlagResult{Flag: true, Data: dest}
}

func (a *App) RestoreBackup(src string) FlagResult {
	if strings.TrimSpace(src) == "" {
		return FlagResult{Flag: false, Data: "请选择备份文件"}
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(backupRoot(), src)
	}
	if err := restoreBackupZip(backupRoot(), src); err != nil {
		return FlagResult{Flag: false, Data: err.Error()}
	}
	return FlagResult{Flag: true, Data: "已恢复，请重启软件"}
}

func (a *App) AutoBackup() FlagResult {
	dest, err := rotateAutoBackups(backupRoot())
	if err != nil {
		return FlagResult{Flag: false, Data: err.Error()}
	}
	return FlagResult{Flag: true, Data: dest}
}

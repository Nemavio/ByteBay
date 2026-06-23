package files

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bytebay/bytebay/engine/internal/config"
)

func init() {
	_ = mime.AddExtensionType(".heic", "image/heic")
	_ = mime.AddExtensionType(".heif", "image/heif")
	_ = mime.AddExtensionType(".avif", "image/avif")
	_ = mime.AddExtensionType(".webp", "image/webp")
	_ = mime.AddExtensionType(".mkv", "video/x-matroska")
	_ = mime.AddExtensionType(".mov", "video/quicktime")
	_ = mime.AddExtensionType(".avi", "video/x-msvideo")
	_ = mime.AddExtensionType(".flac", "audio/flac")
	_ = mime.AddExtensionType(".opus", "audio/opus")
	_ = mime.AddExtensionType(".aac", "audio/aac")
	_ = mime.AddExtensionType(".m4a", "audio/mp4")
}

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mime    string `json:"mime,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

func resolve(path string) (string, error) {
	if path == "" {
		path = config.VolumesRoot
	}
	clean := filepath.Clean(path)
	allowed := false
	for _, root := range []string{config.DataRoot, config.VolumesRoot} {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("path must be under %s or %s", config.DataRoot, config.VolumesRoot)
	}
	return clean, nil
}

func List(path string) ([]Entry, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	var out []Entry
	roots := []string{config.DataRoot, config.VolumesRoot}
	atRoot := false
	for _, root := range roots {
		if p == root {
			atRoot = true
			break
		}
	}
	if !atRoot {
		out = append(out, Entry{Name: "..", Path: filepath.Dir(p), IsDir: true})
	}
	for _, e := range entries {
		full := filepath.Join(p, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		ent := Entry{
			Name:    e.Name(),
			Path:    full,
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		}
		if !e.IsDir() {
			ent.Mime = mime.TypeByExtension(filepath.Ext(e.Name()))
		}
		out = append(out, ent)
	}
	if out == nil {
		out = []Entry{}
	}
	return out, nil
}

func Mkdir(path string) error {
	p, err := resolve(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(p, 0o755)
}

func SaveUpload(path string, r io.Reader) error {
	p, err := resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func Stat(path string) (*Entry, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	ent := &Entry{
		Name:    info.Name(),
		Path:    p,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime().UTC().Format(time.RFC3339),
	}
	if !info.IsDir() {
		ent.Mime = mime.TypeByExtension(filepath.Ext(info.Name()))
	}
	return ent, nil
}

func Delete(path string) error {
	p, err := resolve(path)
	if err != nil {
		return err
	}
	for _, root := range []string{config.DataRoot, config.VolumesRoot} {
		if p == root {
			return fmt.Errorf("cannot delete root %s", root)
		}
	}
	info, err := os.Stat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(p)
	}
	return os.Remove(p)
}

func Move(src, dst string) error {
	sp, err := resolve(src)
	if err != nil {
		return err
	}
	dp, err := resolve(dst)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dp), 0o755); err != nil {
		return err
	}
	return os.Rename(sp, dp)
}

func Open(path string) (*os.File, os.FileInfo, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, nil, fmt.Errorf("is a directory")
	}
	return f, info, nil
}

func Serve(w http.ResponseWriter, path string) error {
	f, info, err := Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if ct := mime.TypeByExtension(filepath.Ext(info.Name())); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, nil, info.Name(), info.ModTime(), f)
	return nil
}

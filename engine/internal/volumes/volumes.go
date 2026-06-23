package volumes

import (
	"os"
	"path/filepath"

	"github.com/bytebay/bytebay/engine/internal/config"
)

type Volume struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func List() ([]Volume, error) {
	root := config.VolumesRoot
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Volume{}, nil
		}
		return nil, err
	}
	var out []Volume
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, Volume{
			Name: e.Name(),
			Path: filepath.Join(root, e.Name()),
		})
	}
	if out == nil {
		out = []Volume{}
	}
	return out, nil
}

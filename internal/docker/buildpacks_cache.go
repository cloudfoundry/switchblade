package docker

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

type BuildpacksCache struct {
	workspace string
	locks     string
	index     *sync.Map
}

func NewBuildpacksCache(workspace, locks string) BuildpacksCache {
	return BuildpacksCache{
		workspace: workspace,
		locks:     locks,
		index:     &sync.Map{},
	}
}

func (c BuildpacksCache) Fetch(uri string) (io.ReadCloser, error) {
	err := os.MkdirAll(c.workspace, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uri: %w", err)
	}

	if !u.IsAbs() {
		file, err := os.Open(uri)
		if err != nil {
			return nil, fmt.Errorf("failed to open buildpack: %w", err)
		}

		return file, nil
	}

	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(uri)))
	path := filepath.Join(c.workspace, sum)

	value, _ := c.index.LoadOrStore(path, flock.New(fmt.Sprintf("%s.lock", filepath.Join(c.locks, sum))))
	mutex := value.(*flock.Flock)

	if err := mutex.Lock(); err != nil {
		return nil, err
	}
	defer mutex.Unlock() //nolint:errcheck

	_, err = os.Stat(path)
	if err == nil {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open buildpack: %w", err)
		}

		return file, nil
	}

	resp, err := http.Get(uri) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to download buildpack: %w", err)
	}
	defer resp.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create buildpack file: %w", err)
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to copy buildpack file: %w", err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to rewind buildpack file: %w", err)
	}

	return file, nil
}

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
)

type BuildpacksCache struct {
	workspace string
	index     *sync.Map
}

func NewBuildpacksCache(workspace string) BuildpacksCache {
	return BuildpacksCache{
		workspace: workspace,
		index:     &sync.Map{},
	}
}

func (c BuildpacksCache) Fetch(uri string) (io.ReadCloser, error) {
	err := os.MkdirAll(c.workspace, os.ModePerm)
	if err != nil {
		panic(err)
	}

	u, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}

	if !u.IsAbs() {
		file, err := os.Open(uri)
		if err != nil {
			panic(err)
		}

		return file, nil
	}

	path := filepath.Join(c.workspace, fmt.Sprintf("%x", sha256.Sum256([]byte(uri))))

	value, _ := c.index.LoadOrStore(path, &sync.Mutex{})
	mutex, ok := value.(*sync.Mutex)
	if !ok {
		panic("something bad happened")
	}

	mutex.Lock()
	defer mutex.Unlock()

	_, err = os.Stat(path)
	if err == nil {
		file, err := os.Open(path)
		if err != nil {
			panic(err)
		}

		return file, nil
	}

	resp, err := http.Get(uri)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		panic(err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		panic(err)
	}

	return file, nil
}

package docker

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/vacation"
)

//go:generate faux --interface Executable --output fakes/executable.go
type Executable interface {
	Execute(pexec.Execution) error
}

type LifecycleManager struct {
	golang   Executable
	archiver Archiver
	m        *sync.Mutex
}

func NewLifecycleManager(golang Executable, archiver Archiver) LifecycleManager {
	return LifecycleManager{
		golang:   golang,
		archiver: archiver,
		m:        &sync.Mutex{},
	}
}

func (b LifecycleManager) Build(sourceURI, workspace string) (string, error) {
	b.m.Lock()
	defer b.m.Unlock()

	req, err := http.NewRequest("GET", sourceURI, nil)
	if err != nil {
		panic(err)
	}

	etag, err := os.ReadFile(filepath.Join(workspace, "etag"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	if len(etag) > 0 {
		req.Header.Set("If-None-Match", string(etag))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	output := filepath.Join(workspace, "lifecycle.tar.gz")
	if resp.StatusCode == http.StatusNotModified {
		return output, nil
	}

	err = os.RemoveAll(workspace)
	if err != nil {
		panic(err)
	}

	err = vacation.NewZipArchive(resp.Body).StripComponents(1).Decompress(filepath.Join(workspace, "repo"))
	if err != nil {
		panic(err)
	}

	env := append(os.Environ(), "GOOS=linux", "GOARCH=amd64")

	_, err = os.Stat(filepath.Join(workspace, "repo", "go.mod"))
	if errors.Is(err, os.ErrNotExist) {
		err = b.golang.Execute(pexec.Execution{
			Args:   []string{"mod", "init", "code.cloudfoundry.org/buildpackapplifecycle"},
			Env:    env,
			Dir:    filepath.Join(workspace, "repo"),
			Stdout: os.Stdout, // TODO: remove
			Stderr: os.Stderr, // TODO: remove
		})
		if err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	err = b.golang.Execute(pexec.Execution{
		Args:   []string{"mod", "tidy"},
		Env:    env,
		Dir:    filepath.Join(workspace, "repo"),
		Stdout: os.Stdout, // TODO: remove
		Stderr: os.Stderr, // TODO: remove
	})
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(filepath.Join(workspace, "output"), os.ModePerm)
	if err != nil {
		panic(err)
	}

	err = b.golang.Execute(pexec.Execution{
		Args:   []string{"build", "-o", filepath.Join(workspace, "output", "builder"), "./builder"},
		Env:    env,
		Dir:    filepath.Join(workspace, "repo"),
		Stdout: os.Stdout, // TODO: remove
		Stderr: os.Stderr, // TODO: remove
	})
	if err != nil {
		panic(err)
	}

	err = b.golang.Execute(pexec.Execution{
		Args:   []string{"build", "-o", filepath.Join(workspace, "output", "launcher"), "./launcher"},
		Env:    env,
		Dir:    filepath.Join(workspace, "repo"),
		Stdout: os.Stdout, // TODO: remove
		Stderr: os.Stderr, // TODO: remove
	})
	if err != nil {
		panic(err)
	}

	err = b.archiver.WithPrefix("/tmp/lifecycle").Compress(filepath.Join(workspace, "output"), output)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filepath.Join(workspace, "etag"), []byte(resp.Header.Get("ETag")), 0600)
	if err != nil {
		panic(err)
	}

	return output, nil
}

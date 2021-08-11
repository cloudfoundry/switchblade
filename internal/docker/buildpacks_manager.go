package docker

import (
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit/vacation"
)

//go:generate faux --interface BPCache --output fakes/bp_cache.go
type BPCache interface {
	Fetch(url string) (io.ReadCloser, error)
}

//go:generate faux --interface BPRegistry --output fakes/bp_registry.go
type BPRegistry interface {
	List() ([]Buildpack, error)
	Override(...Buildpack)
}

type Buildpack struct {
	Name string
	URI  string
}

type BuildpacksManager struct {
	archiver Archiver
	cache    BPCache
	registry BPRegistry

	filter []string
}

func NewBuildpacksManager(archiver Archiver, cache BPCache, registry BPRegistry) BuildpacksManager {
	return BuildpacksManager{
		archiver: archiver,
		cache:    cache,
		registry: registry,
	}
}

func (m BuildpacksManager) Build(workspace, name string) (string, error) {
	buildpacks, err := m.registry.List()
	if err != nil {
		return "", fmt.Errorf("failed to list buildpacks: %w", err)
	}

	for _, buildpack := range buildpacks {
		contains := len(m.filter) == 0
		for _, name := range m.filter {
			if buildpack.Name == name {
				contains = true
				break
			}
		}

		if !contains {
			continue
		}

		bp, err := m.cache.Fetch(buildpack.URI)
		if err != nil {
			return "", fmt.Errorf("failed to fetch buildpack: %w", err)
		}

		err = vacation.NewZipArchive(bp).Decompress(filepath.Join(workspace, name, fmt.Sprintf("%x", md5.Sum([]byte(buildpack.Name)))))
		if err != nil {
			return "", fmt.Errorf("failed to decompress buildpack: %w", err)
		}

		err = bp.Close()
		if err != nil {
			return "", fmt.Errorf("failed to close buildpack: %w", err)
		}
	}

	output := filepath.Join(workspace, fmt.Sprintf("%s.tar.gz", name))
	err = m.archiver.WithPrefix("/tmp/buildpacks").Compress(filepath.Join(workspace, name), output)
	if err != nil {
		return "", fmt.Errorf("failed to archive buildpacks: %w", err)
	}

	return output, nil
}

func (m BuildpacksManager) Order() (string, bool, error) {
	var names []string
	buildpacks, err := m.registry.List()
	if err != nil {
		return "", false, fmt.Errorf("failed to list buildpacks: %w", err)
	}

	if len(m.filter) > 0 {
		names = m.filter
	} else {
		for _, buildpack := range buildpacks {
			names = append(names, buildpack.Name)
		}
	}

	return strings.Join(names, ","), len(m.filter) > 0, nil
}

func (m BuildpacksManager) WithBuildpacks(buildpacks ...string) BuildpacksBuilder {
	m.filter = buildpacks
	return m
}

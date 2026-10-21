package docker_test

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/cloudfoundry/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuildpacksManager(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		manager   docker.BuildpacksManager
		archiver  *fakes.Archiver
		cache     *fakes.BPCache
		registry  *fakes.BPRegistry
		root      string
		workspace string

		cacheFetchInvocations []cacheFetchInvocation
	)

	it.Before(func() {
		var err error
		root, err = os.MkdirTemp("", "workspace")
		Expect(err).NotTo(HaveOccurred())

		// workspace mirrors the "buildpacks" subdirectory Setup.Run passes
		// to BuildpacksManager.Build, so sharedDir (a sibling of workspace)
		// lands next to it inside root rather than leaking into the
		// system temp directory.
		workspace = filepath.Join(root, "buildpacks")
		Expect(os.MkdirAll(workspace, os.ModePerm)).To(Succeed())

		Expect(os.Mkdir(filepath.Join(workspace, "some-buildpack"), os.ModePerm)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(workspace, "some-buildpack", "some-file"), []byte("some-content"), 0600)).To(Succeed())

		Expect(os.Mkdir(filepath.Join(workspace, "some-app"), os.ModePerm)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(workspace, "some-app", "some-file"), []byte("some-content"), 0600)).To(Succeed())

		archiver = &fakes.Archiver{}
		archiver.WithPrefixCall.Returns.Archiver = archiver

		cache = &fakes.BPCache{}
		cache.FetchCall.Stub = func(url string) (io.ReadCloser, error) {
			cacheFetchInvocations = append(cacheFetchInvocations, cacheFetchInvocation{URL: url})

			if info, err := os.Stat(url); err == nil && info.IsDir() {
				fd, err := os.Open(url)
				if err != nil {
					return nil, err
				}

				return fd, nil
			}

			buffer := bytes.NewBuffer(nil)
			writer := zip.NewWriter(buffer)
			defer writer.Close()

			name := strings.TrimSuffix(strings.TrimPrefix(url, "some-"), "-uri")
			f, err := writer.Create(fmt.Sprintf("some-%s-file", name))
			if err != nil {
				return nil, err
			}

			_, err = f.Write([]byte(fmt.Sprintf("some-%s-content", name)))
			if err != nil {
				return nil, err
			}

			return io.NopCloser(buffer), nil
		}

		registry = &fakes.BPRegistry{}
		registry.ListCall.Returns.BuildpackSlice = []docker.Buildpack{
			{
				Name: "ruby-buildpack",
				URI:  "some-ruby-uri",
			},
			{
				Name: "go-buildpack",
				URI:  "some-go-uri",
			},
			{
				Name: "directory-buildpack",
				URI:  filepath.Join(workspace, "some-buildpack"),
			},
			{
				Name: "nodejs-buildpack",
				URI:  "some-nodejs-uri",
			},
		}

		manager = docker.NewBuildpacksManager(archiver, cache, registry)
	})

	it.After(func() {
		Expect(os.RemoveAll(root)).To(Succeed())
	})

	context("Build", func() {
		var (
			compressed []map[string]string

			allBuildpacks = map[string]string{
				filepath.Join("fb563133b31055c118e0f46f44578ed9", "some-ruby-file"):   "some-ruby-content",
				filepath.Join("01013f7c8d79af6e84e9b66bc3645322", "some-go-file"):     "some-go-content",
				filepath.Join("3bec7f3d485eee8707d275dbf41de4d5", "some-nodejs-file"): "some-nodejs-content",
				filepath.Join("39d7879e97b51ef2020898a6a966a915", "some-file"):        "some-content",
			}
		)

		sharedTarballs := func() []string {
			paths, err := filepath.Glob(filepath.Join(root, "buildpacks-shared", "*.tar.gz"))
			Expect(err).NotTo(HaveOccurred())
			return paths
		}

		it.Before(func() {
			compressed = nil
			archiver.CompressCall.Stub = func(input, output string) error {
				snapshot := map[string]string{}
				err := filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}

					rel, err := filepath.Rel(input, path)
					if err != nil {
						return err
					}

					content, err := os.ReadFile(path)
					if err != nil {
						return err
					}

					snapshot[rel] = string(content)
					return nil
				})
				if err != nil {
					return err
				}

				compressed = append(compressed, snapshot)
				return os.WriteFile(output, []byte(fmt.Sprintf("tarball-%d", len(compressed))), 0600)
			}
		})

		it("bundles the buildpacks into a shared tarball and links it for the app", func() {
			buildpacks, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(buildpacks).To(Equal(filepath.Join(workspace, "some-app.tar.gz")))

			Expect(cacheFetchInvocations).To(Equal([]cacheFetchInvocation{
				{URL: "some-ruby-uri"},
				{URL: "some-go-uri"},
				{URL: filepath.Join(workspace, "some-buildpack")},
				{URL: "some-nodejs-uri"},
			}))

			Expect(archiver.WithPrefixCall.Receives.Prefix).To(Equal("/tmp/buildpacks"))
			Expect(archiver.CompressCall.Receives.Input).To(MatchRegexp(`^%s/[0-9a-f]{64}\.[0-9]+-[0-9]+\.staging$`, regexp.QuoteMeta(filepath.Join(root, "buildpacks-shared"))))
			Expect(archiver.CompressCall.Receives.Output).To(MatchRegexp(`^%s/[0-9a-f]{64}\.tar\.gz\.[0-9]+-[0-9]+\.tmp$`, regexp.QuoteMeta(filepath.Join(root, "buildpacks-shared"))))
			Expect(compressed).To(Equal([]map[string]string{allBuildpacks}))

			shared := sharedTarballs()
			Expect(shared).To(HaveLen(1))

			sharedInfo, err := os.Stat(shared[0])
			Expect(err).NotTo(HaveOccurred())
			outputInfo, err := os.Stat(buildpacks)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.SameFile(sharedInfo, outputInfo)).To(BeTrue())

			leftovers, err := filepath.Glob(filepath.Join(root, "buildpacks-shared", "*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(leftovers).To(Equal(shared))
			Expect(filepath.Join(workspace, "some-app", "fb563133b31055c118e0f46f44578ed9")).NotTo(BeADirectory())
		})

		it("reuses the shared tarball for other apps", func() {
			first, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			second, err := manager.Build(workspace, "other-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(second).To(Equal(filepath.Join(workspace, "other-app.tar.gz")))

			Expect(archiver.CompressCall.CallCount).To(Equal(1))
			Expect(cacheFetchInvocations).To(HaveLen(4))
			Expect(sharedTarballs()).To(HaveLen(1))

			content, err := os.ReadFile(first)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("tarball-1"))

			content, err = os.ReadFile(second)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("tarball-1"))
		})

		it("builds the shared tarball once for concurrent builds", func() {
			var wg sync.WaitGroup
			errs := make(chan error, 8)
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, err := manager.Build(workspace, fmt.Sprintf("app-%d", i))
					errs <- err
				}(i)
			}
			wg.Wait()
			close(errs)

			for err := range errs {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(archiver.CompressCall.CallCount).To(Equal(1))
			for i := 0; i < 8; i++ {
				Expect(filepath.Join(workspace, fmt.Sprintf("app-%d.tar.gz", i))).To(BeARegularFile())
			}
		})

		it("rebuilds when a file in a local directory buildpack changes", func() {
			Expect(os.MkdirAll(filepath.Join(workspace, "some-buildpack", "nested"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(workspace, "some-buildpack", "nested", "some-file"), []byte("some-content"), 0600)).To(Succeed())

			_, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.WriteFile(filepath.Join(workspace, "some-buildpack", "nested", "some-file"), []byte("other-content"), 0600)).To(Succeed())

			_, err = manager.Build(workspace, "other-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(archiver.CompressCall.CallCount).To(Equal(2))
			Expect(compressed[1]).To(HaveKeyWithValue(filepath.Join("39d7879e97b51ef2020898a6a966a915", "nested", "some-file"), "other-content"))
		})

		it("bundles the full contents of a symlinked directory buildpack, not just a symlink entry", func() {
			realDir := filepath.Join(workspace, "real-buildpack-dir")
			Expect(os.MkdirAll(filepath.Join(realDir, "nested"), os.ModePerm)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(realDir, "nested", "some-file"), []byte("some-content"), 0600)).To(Succeed())

			symlinkedDir := filepath.Join(workspace, "symlinked-buildpack-dir")
			Expect(os.Symlink(realDir, symlinkedDir)).To(Succeed())

			registry.ListCall.Returns.BuildpackSlice = []docker.Buildpack{
				{
					Name: "symlinked-buildpack",
					URI:  symlinkedDir,
				},
			}

			_, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(compressed).To(HaveLen(1))
			destination := fmt.Sprintf("%x", md5.Sum([]byte("symlinked-buildpack")))
			Expect(compressed[0]).To(HaveKeyWithValue(filepath.Join(destination, "nested", "some-file"), "some-content"))
		})

		it("rebuilds when the shared tarball was removed", func() {
			_, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			for _, path := range sharedTarballs() {
				Expect(os.Remove(path)).To(Succeed())
			}

			_, err = manager.Build(workspace, "other-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(archiver.CompressCall.CallCount).To(Equal(2))
			Expect(filepath.Join(workspace, "other-app.tar.gz")).To(BeARegularFile())
		})

		it("keeps only the most recently used shared tarballs", func() {
			var built []string
			for i, name := range []string{"ruby-buildpack", "go-buildpack", "nodejs-buildpack", "directory-buildpack"} {
				before := sharedTarballs()

				_, err := manager.WithBuildpacks(name).Build(workspace, fmt.Sprintf("app-%d", i))
				Expect(err).NotTo(HaveOccurred())

				for _, path := range sharedTarballs() {
					if !slices.Contains(before, path) {
						built = append(built, path)
						modified := time.Now().Add(time.Duration(i-10) * time.Minute)
						Expect(os.Chtimes(path, modified, modified)).To(Succeed())
					}
				}
			}

			Expect(built).To(HaveLen(4))
			Expect(sharedTarballs()).To(ConsistOf(built[1:]))
		})

		it("reuses the shared tarball when the registry lists buildpacks in another order", func() {
			_, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			slices.Reverse(registry.ListCall.Returns.BuildpackSlice)

			_, err = manager.Build(workspace, "other-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(archiver.CompressCall.CallCount).To(Equal(1))
		})

		it("removes stale temporary files when reusing the shared tarball", func() {
			_, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			stale := filepath.Join(root, "buildpacks-shared", "0000000000000000.1-1.staging")
			Expect(os.Mkdir(stale, os.ModePerm)).To(Succeed())
			old := time.Now().Add(-2 * time.Hour)
			Expect(os.Chtimes(stale, old, old)).To(Succeed())

			_, err = manager.Build(workspace, "other-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(archiver.CompressCall.CallCount).To(Equal(1))
			Expect(stale).NotTo(BeADirectory())
		})

		it("removes stale temporary files but keeps recent ones", func() {
			Expect(os.MkdirAll(filepath.Join(root, "buildpacks-shared"), os.ModePerm)).To(Succeed())

			stale := filepath.Join(root, "buildpacks-shared", "0000000000000000.1-1.staging")
			Expect(os.Mkdir(stale, os.ModePerm)).To(Succeed())
			old := time.Now().Add(-2 * time.Hour)
			Expect(os.Chtimes(stale, old, old)).To(Succeed())

			recent := filepath.Join(root, "buildpacks-shared", "1111111111111111.tar.gz.1-2.tmp")
			Expect(os.WriteFile(recent, []byte("in progress"), 0600)).To(Succeed())

			_, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())

			Expect(stale).NotTo(BeADirectory())
			Expect(recent).To(BeARegularFile())
		})

		context("WithBuildpacks", func() {
			it("only builds the named buildpacks", func() {
				_, err := manager.WithBuildpacks("ruby-buildpack", "nodejs-buildpack").Build(workspace, "some-app")
				Expect(err).NotTo(HaveOccurred())

				Expect(compressed).To(Equal([]map[string]string{{
					filepath.Join("fb563133b31055c118e0f46f44578ed9", "some-ruby-file"):   "some-ruby-content",
					filepath.Join("3bec7f3d485eee8707d275dbf41de4d5", "some-nodejs-file"): "some-nodejs-content",
				}}))
			})

			it("builds a separate shared tarball per set of buildpacks", func() {
				_, err := manager.WithBuildpacks("ruby-buildpack").Build(workspace, "some-app")
				Expect(err).NotTo(HaveOccurred())

				_, err = manager.WithBuildpacks("go-buildpack").Build(workspace, "other-app")
				Expect(err).NotTo(HaveOccurred())

				Expect(archiver.CompressCall.CallCount).To(Equal(2))
				Expect(sharedTarballs()).To(HaveLen(2))
			})
		})

		context("WithoutSharing", func() {
			it("rebuilds the tarball for every app", func() {
				buildpacks, err := manager.WithoutSharing().Build(workspace, "some-app")
				Expect(err).NotTo(HaveOccurred())
				Expect(buildpacks).To(Equal(filepath.Join(workspace, "some-app.tar.gz")))

				Expect(archiver.CompressCall.Receives.Input).To(Equal(filepath.Join(workspace, "some-app")))
				Expect(archiver.CompressCall.Receives.Output).To(Equal(filepath.Join(workspace, "some-app.tar.gz")))
				Expect(compressed).To(Equal([]map[string]string{allBuildpacks}))

				_, err = manager.WithoutSharing().Build(workspace, "other-app")
				Expect(err).NotTo(HaveOccurred())

				Expect(archiver.CompressCall.CallCount).To(Equal(2))
				Expect(sharedTarballs()).To(BeEmpty())
			})

			it("does not corrupt a shared tarball that another app is still linked to", func() {
				// some-app and other-app share the same tarball via a hard link.
				someApp, err := manager.Build(workspace, "some-app")
				Expect(err).NotTo(HaveOccurred())

				otherApp, err := manager.Build(workspace, "other-app")
				Expect(err).NotTo(HaveOccurred())

				shared := sharedTarballs()
				Expect(shared).To(HaveLen(1))

				sharedInfo, err := os.Stat(shared[0])
				Expect(err).NotTo(HaveOccurred())
				someAppInfo, err := os.Stat(someApp)
				Expect(err).NotTo(HaveOccurred())
				Expect(os.SameFile(sharedInfo, someAppInfo)).To(BeTrue())

				// Rebuilding some-app without sharing must not truncate the
				// shared tarball in place, which would corrupt it for
				// other-app and the shared cache itself.
				_, err = manager.WithoutSharing().Build(workspace, "some-app")
				Expect(err).NotTo(HaveOccurred())

				content, err := os.ReadFile(otherApp)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("tarball-1"))

				content, err = os.ReadFile(shared[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("tarball-1"))
			})
		})

		context("when an app is named \"shared\"", func() {
			it("rejects the name instead of risking collision with the shared tarball directory", func() {
				_, err := manager.Build(workspace, "shared")
				Expect(err).To(MatchError(ContainSubstring(`name "shared" is reserved`)))

				_, err = manager.Build(workspace, "other-app")
				Expect(err).NotTo(HaveOccurred())

				Expect(archiver.CompressCall.CallCount).To(Equal(1))
				Expect(sharedTarballs()).To(HaveLen(1))
			})
		})

		context("failure cases", func() {
			context("when the registry cannot list the buildpacks", func() {
				it.Before(func() {
					registry.ListCall.Returns.Error = errors.New("could not list buildpacks")
				})

				it("returns an error", func() {
					_, err := manager.Build(workspace, "some-app")
					Expect(err).To(MatchError("failed to list buildpacks: could not list buildpacks"))
				})
			})

			context("when the cache cannot fetch the buildpack", func() {
				it.Before(func() {
					cache.FetchCall.Stub = nil
					cache.FetchCall.Returns.Error = errors.New("could not fetch buildpack")
				})

				it("returns an error", func() {
					_, err := manager.Build(workspace, "some-app")
					Expect(err).To(MatchError("failed to fetch buildpack: could not fetch buildpack"))
				})
			})

			context("when a directory buildpack cannot be copied", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(workspace, "some-buildpack", "some-file"), 0000)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := manager.Build(workspace, "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to copy buildpack:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the buildpack cannot be decompressed", func() {
				it.Before(func() {
					cache.FetchCall.Stub = func(url string) (io.ReadCloser, error) {
						return io.NopCloser(bytes.NewBuffer([]byte("this is not a zip file"))), nil
					}
				})

				it("returns an error", func() {
					_, err := manager.Build(workspace, "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to decompress buildpack:")))
					Expect(err).To(MatchError(ContainSubstring("not a valid zip file")))
				})
			})

			context("when the archiver fails to compress the buildpacks", func() {
				it.Before(func() {
					archiver.CompressCall.Stub = nil
					archiver.CompressCall.Returns.Error = errors.New("could not compress buildpacks")
				})

				it("returns an error", func() {
					_, err := manager.Build(workspace, "some-app")
					Expect(err).To(MatchError("failed to archive buildpacks: could not compress buildpacks"))

					leftovers, err := filepath.Glob(filepath.Join(root, "buildpacks-shared", "*"))
					Expect(err).NotTo(HaveOccurred())
					Expect(leftovers).To(BeEmpty())
				})
			})
		})
	})

	context("Order", func() {
		it("returns a comma-separated list of the buildpacks", func() {
			order, skipDetect, err := manager.Order()
			Expect(err).NotTo(HaveOccurred())
			Expect(order).To(Equal("ruby-buildpack,go-buildpack,directory-buildpack,nodejs-buildpack"))
			Expect(skipDetect).To(BeFalse())
		})

		context("WithBuildpacks", func() {
			it("only returns those named buildpacks", func() {
				order, skipDetect, err := manager.WithBuildpacks("nodejs-buildpack", "go-buildpack").Order()
				Expect(err).NotTo(HaveOccurred())
				Expect(order).To(Equal("nodejs-buildpack,go-buildpack"))
				Expect(skipDetect).To(BeTrue())
			})
		})

		context("failure cases", func() {
			context("when the registry cannot list the buildpacks", func() {
				it.Before(func() {
					registry.ListCall.Returns.Error = errors.New("could not list buildpacks")
				})

				it("returns an error", func() {
					_, _, err := manager.Order()
					Expect(err).To(MatchError("failed to list buildpacks: could not list buildpacks"))
				})
			})
		})
	})
}

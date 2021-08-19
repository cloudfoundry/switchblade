package docker_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
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
		workspace string

		cacheFetchInvocations []cacheFetchInvocation
	)

	it.Before(func() {
		var err error
		workspace, err = os.MkdirTemp("", "workspace")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workspace, "some-buildpack"), os.ModePerm)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(workspace, "some-buildpack", "some-file"), []byte("some-content"), 0600)).To(Succeed())

		archiver = &fakes.Archiver{}
		archiver.WithPrefixCall.Returns.Archiver = archiver

		cache = &fakes.BPCache{}
		cache.FetchCall.Stub = func(url string) (io.ReadCloser, error) {
			cacheFetchInvocations = append(cacheFetchInvocations, cacheFetchInvocation{URL: url})

			if url == filepath.Join(workspace, "some-buildpack") {
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
		Expect(os.RemoveAll(workspace)).To(Succeed())
	})

	context("Build", func() {
		it("bundles the buildpacks into a tarball", func() {
			buildpacks, err := manager.Build(workspace, "some-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(buildpacks).To(Equal(filepath.Join(workspace, "some-app.tar.gz")))

			Expect(cacheFetchInvocations).To(HaveLen(4))
			Expect(cacheFetchInvocations[0]).To(Equal(cacheFetchInvocation{
				URL: "some-ruby-uri",
			}))
			Expect(cacheFetchInvocations[1]).To(Equal(cacheFetchInvocation{
				URL: "some-go-uri",
			}))
			Expect(cacheFetchInvocations[2]).To(Equal(cacheFetchInvocation{
				URL: filepath.Join(workspace, "some-buildpack"),
			}))
			Expect(cacheFetchInvocations[3]).To(Equal(cacheFetchInvocation{
				URL: "some-nodejs-uri",
			}))

			Expect(archiver.WithPrefixCall.Receives.Prefix).To(Equal("/tmp/buildpacks"))
			Expect(archiver.CompressCall.Receives.Input).To(Equal(filepath.Join(workspace, "some-app")))
			Expect(archiver.CompressCall.Receives.Output).To(Equal(filepath.Join(workspace, "some-app.tar.gz")))

			directories, err := filepath.Glob(filepath.Join(workspace, "some-app", "*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(directories).To(ConsistOf([]string{
				filepath.Join(workspace, "some-app", "fb563133b31055c118e0f46f44578ed9"),
				filepath.Join(workspace, "some-app", "01013f7c8d79af6e84e9b66bc3645322"),
				filepath.Join(workspace, "some-app", "3bec7f3d485eee8707d275dbf41de4d5"),
				filepath.Join(workspace, "some-app", "39d7879e97b51ef2020898a6a966a915"),
			}))

			content, err := os.ReadFile(filepath.Join(workspace, "some-app", "fb563133b31055c118e0f46f44578ed9", "some-ruby-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-ruby-content"))

			content, err = os.ReadFile(filepath.Join(workspace, "some-app", "01013f7c8d79af6e84e9b66bc3645322", "some-go-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-go-content"))

			content, err = os.ReadFile(filepath.Join(workspace, "some-app", "3bec7f3d485eee8707d275dbf41de4d5", "some-nodejs-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-nodejs-content"))

			content, err = os.ReadFile(filepath.Join(workspace, "some-app", "39d7879e97b51ef2020898a6a966a915", "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-content"))
		})

		context("WithBuildpacks", func() {
			it("only builds the named buildpacks", func() {
				_, err := manager.WithBuildpacks("ruby-buildpack", "nodejs-buildpack").Build(workspace, "some-app")
				Expect(err).NotTo(HaveOccurred())

				directories, err := filepath.Glob(filepath.Join(workspace, "some-app", "*"))
				Expect(err).NotTo(HaveOccurred())
				Expect(directories).To(ConsistOf([]string{
					filepath.Join(workspace, "some-app", "fb563133b31055c118e0f46f44578ed9"),
					filepath.Join(workspace, "some-app", "3bec7f3d485eee8707d275dbf41de4d5"),
				}))

				content, err := os.ReadFile(filepath.Join(workspace, "some-app", "fb563133b31055c118e0f46f44578ed9", "some-ruby-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-ruby-content"))

				content, err = os.ReadFile(filepath.Join(workspace, "some-app", "3bec7f3d485eee8707d275dbf41de4d5", "some-nodejs-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-nodejs-content"))
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
					archiver.CompressCall.Returns.Error = errors.New("could not compress buildpacks")
				})

				it("returns an error", func() {
					_, err := manager.Build(workspace, "some-app")
					Expect(err).To(MatchError("failed to archive buildpacks: could not compress buildpacks"))
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

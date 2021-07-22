package docker_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuildpacksCache(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		cache     docker.BuildpacksCache
		server    *httptest.Server
		workspace string
		sum       string
	)

	it.Before(func() {
		var err error
		workspace, err = os.MkdirTemp("", "workspace")
		Expect(err).NotTo(HaveOccurred())

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprintf(w, "some-content")
		}))

		sum = fmt.Sprintf("%x", sha256.Sum256([]byte(server.URL)))

		cache = docker.NewBuildpacksCache(filepath.Join(workspace, "some-cache"))
	})

	it.After(func() {
		Expect(os.RemoveAll(workspace)).To(Succeed())
	})

	it("downloads the buildpack into the cache", func() {
		buildpack, err := cache.Fetch(server.URL)
		Expect(err).NotTo(HaveOccurred())

		content, err := io.ReadAll(buildpack)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("some-content"))

		Expect(buildpack.Close()).To(Succeed())

		Expect(filepath.Join(workspace, "some-cache", sum)).To(BeAnExistingFile())

		content, err = os.ReadFile(filepath.Join(workspace, "some-cache", sum))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("some-content"))
	})

	context("when the buildpack is already in the cache", func() {
		it.Before(func() {
			err := os.Mkdir(filepath.Join(workspace, "some-cache"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(filepath.Join(workspace, "some-cache", sum), []byte("cached-content"), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it("reuses the cached buildpack", func() {
			buildpack, err := cache.Fetch(server.URL)
			Expect(err).NotTo(HaveOccurred())

			content, err := io.ReadAll(buildpack)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("cached-content"))

			Expect(buildpack.Close()).To(Succeed())
		})
	})

	context("when the url is a filepath", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(workspace, "some-buildpack"), []byte("file-content"), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it("returns the filepath", func() {
			buildpack, err := cache.Fetch(filepath.Join(workspace, "some-buildpack"))
			Expect(err).NotTo(HaveOccurred())

			content, err := io.ReadAll(buildpack)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("file-content"))

			Expect(buildpack.Close()).To(Succeed())
		})
	})
}

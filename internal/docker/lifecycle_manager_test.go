package docker_test

import (
	"archive/zip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func testLifecycleManager(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workspace  string
		executable *fakes.Executable
		executions []pexec.Execution
		server     *httptest.Server
		archiver   *fakes.Archiver

		manager docker.LifecycleManager
	)

	it.Before(func() {
		var err error
		workspace, err = os.MkdirTemp("", "workspace")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.WriteFile(filepath.Join(workspace, "extra-file"), nil, 0600)).To(Succeed())

		executable = &fakes.Executable{}
		executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
			executions = append(executions, execution)
			return nil
		}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("ETag", "some-etag")

			if req.Header.Get("If-None-Match") == "some-etag" {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			writer := zip.NewWriter(w)
			_, err := writer.Create("some-dir/")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			f, err := writer.Create(filepath.Join("some-dir", "some-file"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			_, err = f.Write([]byte("repo-content"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if req.URL.Path == "/with-go-modules" {
				f, err := writer.Create(filepath.Join("some-dir", "go.mod"))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				_, err = f.Write([]byte("go-mod-content"))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			err = writer.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}))

		archiver = &fakes.Archiver{}
		archiver.WithPrefixCall.Returns.Archiver = archiver

		manager = docker.NewLifecycleManager(executable, archiver)
	})

	it.After(func() {
		Expect(os.RemoveAll(workspace)).To(Succeed())
	})

	it("builds the lifecycle", func() {
		path, err := manager.Build(server.URL, workspace)
		Expect(err).NotTo(HaveOccurred())
		Expect(path).To(Equal(filepath.Join(workspace, "lifecycle.tar.gz")))

		rel, err := filepath.Rel(workspace, path)
		Expect(err).NotTo(HaveOccurred())
		Expect(rel).To(Equal("lifecycle.tar.gz"))

		Expect(executions).To(HaveLen(4))
		Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
			"Args": Equal([]string{"mod", "init", "code.cloudfoundry.org/buildpackapplifecycle"}),
			"Dir":  Equal(filepath.Join(workspace, "repo")),
		}))
		Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
			"Args": Equal([]string{"mod", "tidy"}),
			"Dir":  Equal(filepath.Join(workspace, "repo")),
		}))
		Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
			"Args": Equal([]string{"build", "-o", filepath.Join(workspace, "output", "builder"), "./builder"}),
			"Env":  ContainElements("GOOS=linux", "GOARCH=amd64"),
			"Dir":  Equal(filepath.Join(workspace, "repo")),
		}))
		Expect(executions[3]).To(MatchFields(IgnoreExtras, Fields{
			"Args": Equal([]string{"build", "-o", filepath.Join(workspace, "output", "launcher"), "./launcher"}),
			"Env":  ContainElements("GOOS=linux", "GOARCH=amd64"),
			"Dir":  Equal(filepath.Join(workspace, "repo")),
		}))

		Expect(archiver.WithPrefixCall.Receives.Prefix).To(Equal("/tmp/lifecycle"))
		Expect(archiver.CompressCall.Receives.Input).To(Equal(filepath.Join(workspace, "output")))
		Expect(archiver.CompressCall.Receives.Output).To(Equal(filepath.Join(workspace, "lifecycle.tar.gz")))

		etag, err := os.ReadFile(filepath.Join(workspace, "etag"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(etag)).To(Equal("some-etag"))

		Expect(filepath.Join(workspace, "extra-file")).NotTo(BeAnExistingFile())
	})

	context("when a go.mod file already exists", func() {
		it("builds the lifecycle", func() {
			path, err := manager.Build(fmt.Sprintf("%s/with-go-modules", server.URL), workspace)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(filepath.Join(workspace, "lifecycle.tar.gz")))

			rel, err := filepath.Rel(workspace, path)
			Expect(err).NotTo(HaveOccurred())
			Expect(rel).To(Equal("lifecycle.tar.gz"))

			Expect(executions).To(HaveLen(3))
			Expect(executions[0]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"mod", "tidy"}),
				"Dir":  Equal(filepath.Join(workspace, "repo")),
			}))
			Expect(executions[1]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"build", "-o", filepath.Join(workspace, "output", "builder"), "./builder"}),
				"Env":  ContainElements("GOOS=linux", "GOARCH=amd64"),
				"Dir":  Equal(filepath.Join(workspace, "repo")),
			}))
			Expect(executions[2]).To(MatchFields(IgnoreExtras, Fields{
				"Args": Equal([]string{"build", "-o", filepath.Join(workspace, "output", "launcher"), "./launcher"}),
				"Env":  ContainElements("GOOS=linux", "GOARCH=amd64"),
				"Dir":  Equal(filepath.Join(workspace, "repo")),
			}))

			Expect(archiver.WithPrefixCall.Receives.Prefix).To(Equal("/tmp/lifecycle"))
			Expect(archiver.CompressCall.Receives.Input).To(Equal(filepath.Join(workspace, "output")))
			Expect(archiver.CompressCall.Receives.Output).To(Equal(filepath.Join(workspace, "lifecycle.tar.gz")))
		})
	})

	context("when the etag matches", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(workspace, "etag"), []byte("some-etag"), 0600)
			Expect(err).NotTo(HaveOccurred())
		})

		it("skips building the lifecycle", func() {
			path, err := manager.Build(server.URL, workspace)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(filepath.Join(workspace, "lifecycle.tar.gz")))

			Expect(executions).To(HaveLen(0))

			Expect(archiver.WithPrefixCall.CallCount).To(Equal(0))
			Expect(archiver.CompressCall.CallCount).To(Equal(0))
		})
	})
}

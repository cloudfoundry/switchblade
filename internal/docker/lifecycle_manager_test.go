package docker_test

import (
	"archive/zip"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/cloudfoundry/switchblade/internal/docker/fakes"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func testLifecycleManager(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Build", func() {
		var (
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

			context("failure cases", func() {
				context("when the etag cannot be read", func() {
					it.Before(func() {
						Expect(os.Chmod(filepath.Join(workspace, "etag"), 0000)).To(Succeed())
					})

					it("returns an error", func() {
						_, err := manager.Build(server.URL, workspace)
						Expect(err).To(MatchError(ContainSubstring("failed to read etag:")))
						Expect(err).To(MatchError(ContainSubstring("permission denied")))
					})
				})
			})
		})

		context("failure cases", func() {
			context("when the source uri is malformed", func() {
				it("returns an error", func() {
					_, err := manager.Build("%%%", workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to create request:")))
					Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
				})
			})

			context("when the request fails", func() {
				it("returns an error", func() {
					_, err := manager.Build("http://localhost:0", workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to complete request:")))
					Expect(err).To(MatchError(ContainSubstring("dial tcp")))
				})
			})

			context("when the response is not a valid zip file", func() {
				it.Before(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						fmt.Fprint(w, "this is not a zip file")
					}))
				})

				it.After(func() {
					server.Close()
				})

				it("returns an error", func() {
					_, err := manager.Build(server.URL, workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to decompress lifecycle repo:")))
					Expect(err).To(MatchError(ContainSubstring("not a valid zip file")))
				})
			})

			context("when initializing the go module fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "mod init") {
							fmt.Fprintln(execution.Stdout, "stdout: could not initialize")
							fmt.Fprintln(execution.Stderr, "stderr: could not initialize")
							return errors.New("go mod init errored")
						}

						return nil
					}
				})

				it("returns an error", func() {
					_, err := manager.Build(server.URL, workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to initialize go module: go mod init errored")))
					Expect(err).To(MatchError(ContainSubstring("stdout: could not initialize")))
					Expect(err).To(MatchError(ContainSubstring("stderr: could not initialize")))
				})
			})

			context("when tidy-ing the go module fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "mod tidy") {
							fmt.Fprintln(execution.Stdout, "stdout: could not tidy")
							fmt.Fprintln(execution.Stderr, "stderr: could not tidy")
							return errors.New("go mod tidy errored")
						}

						return nil
					}
				})

				it("returns an error", func() {
					_, err := manager.Build(server.URL, workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to tidy go module: go mod tidy errored")))
					Expect(err).To(MatchError(ContainSubstring("stdout: could not tidy")))
					Expect(err).To(MatchError(ContainSubstring("stderr: could not tidy")))
				})
			})

			context("when building the builder fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "./builder") {
							fmt.Fprintln(execution.Stdout, "stdout: could not build builder")
							fmt.Fprintln(execution.Stderr, "stderr: could not build builder")
							return errors.New("go build builder errored")
						}

						return nil
					}
				})

				it("returns an error", func() {
					_, err := manager.Build(server.URL, workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to build lifecycle builder: go build builder errored")))
					Expect(err).To(MatchError(ContainSubstring("stdout: could not build builder")))
					Expect(err).To(MatchError(ContainSubstring("stderr: could not build builder")))
				})
			})

			context("when building the launcher fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "./launcher") {
							fmt.Fprintln(execution.Stdout, "stdout: could not build launcher")
							fmt.Fprintln(execution.Stderr, "stderr: could not build launcher")
							return errors.New("go build launcher errored")
						}

						return nil
					}
				})

				it("returns an error", func() {
					_, err := manager.Build(server.URL, workspace)
					Expect(err).To(MatchError(ContainSubstring("failed to build lifecycle launcher: go build launcher errored")))
					Expect(err).To(MatchError(ContainSubstring("stdout: could not build launcher")))
					Expect(err).To(MatchError(ContainSubstring("stderr: could not build launcher")))
				})
			})

			context("when the lifecycle cannot be archived", func() {
				it.Before(func() {
					archiver.CompressCall.Returns.Error = errors.New("could not compress lifecycle")
				})

				it("returns an error", func() {
					_, err := manager.Build(server.URL, workspace)
					Expect(err).To(MatchError("failed to archive lifecycle: could not compress lifecycle"))
				})
			})
		})
	})
}

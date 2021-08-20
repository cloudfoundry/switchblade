package docker_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	gocontext "context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/paketo-buildpacks/packit/vacation"
	"github.com/ryanmoran/switchblade/internal/docker"
	"github.com/ryanmoran/switchblade/internal/docker/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/ryanmoran/switchblade/matchers"
)

func testStage(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Run", func() {
		var (
			stage docker.Stage

			client    *fakes.StageClient
			workspace string

			copyFromContainerInvocations []copyFromContainerInvocation
		)

		it.Before(func() {
			var err error
			workspace, err = os.MkdirTemp("", "workspace")
			Expect(err).NotTo(HaveOccurred())

			client = &fakes.StageClient{}
			containerWaitOKBodyChannel := make(chan container.ContainerWaitOKBody)
			close(containerWaitOKBodyChannel)
			client.ContainerWaitCall.Returns.ContainerWaitOKBodyChannel = containerWaitOKBodyChannel
			containerLogs := bytes.NewBuffer(nil)
			containerLogsWriter := stdcopy.NewStdWriter(containerLogs, stdcopy.Stdout)
			_, err = containerLogsWriter.Write([]byte("Fetching container logs...\n"))
			Expect(err).NotTo(HaveOccurred())
			client.ContainerLogsCall.Returns.ReadCloser = io.NopCloser(containerLogs)
			client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
				copyFromContainerInvocations = append(copyFromContainerInvocations, copyFromContainerInvocation{
					ContainerID: containerID,
					SrcPath:     srcPath,
				})

				buffer := bytes.NewBuffer(nil)
				switch srcPath {
				case "/tmp/droplet":
					if err := generateDroplet(buffer); err != nil {
						return nil, types.ContainerPathStat{}, err
					}

				case "/tmp/output-cache":
					if err := generateBuildCache(buffer); err != nil {
						return nil, types.ContainerPathStat{}, err
					}

				case "/tmp/result.json":
					err := generateResultJSON(buffer, `{
						"processes": [
							{ "type": "web", "command": "some-command" },
							{ "type": "worker", "command": "other-command" }
						]
					}`)
					if err != nil {
						return nil, types.ContainerPathStat{}, err
					}
				}

				return io.NopCloser(buffer), types.ContainerPathStat{}, nil
			}

			archiver := docker.NewTGZArchiver()

			stage = docker.NewStage(client, archiver, workspace)
		})

		it.After(func() {
			Expect(os.RemoveAll(workspace)).To(Succeed())
		})

		it("builds and runs the app", func() {
			ctx := gocontext.Background()
			logs := bytes.NewBuffer(nil)

			command, err := stage.Run(ctx, logs, "some-container-id", "some-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(command).To(Equal("some-command"))

			Expect(client.ContainerStartCall.Receives.ContainerID).To(Equal("some-container-id"))

			Expect(client.ContainerWaitCall.Receives.ContainerID).To(Equal("some-container-id"))
			Expect(client.ContainerWaitCall.Receives.Condition).To(Equal(container.WaitConditionNotRunning))

			Expect(client.ContainerLogsCall.Receives.Container).To(Equal("some-container-id"))
			Expect(client.ContainerLogsCall.Receives.Options).To(Equal(types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			}))

			Expect(copyFromContainerInvocations).To(HaveLen(3))
			Expect(copyFromContainerInvocations[0].ContainerID).To(Equal("some-container-id"))
			Expect(copyFromContainerInvocations[0].SrcPath).To(Equal("/tmp/droplet"))
			Expect(copyFromContainerInvocations[1].ContainerID).To(Equal("some-container-id"))
			Expect(copyFromContainerInvocations[1].SrcPath).To(Equal("/tmp/output-cache"))
			Expect(copyFromContainerInvocations[2].ContainerID).To(Equal("some-container-id"))
			Expect(copyFromContainerInvocations[2].SrcPath).To(Equal("/tmp/result.json"))

			Expect(client.ContainerRemoveCall.Receives.ContainerID).To(Equal("some-container-id"))
			Expect(client.ContainerRemoveCall.Receives.Options).To(Equal(types.ContainerRemoveOptions{Force: true}))

			Expect(logs).To(ContainLines("Fetching container logs..."))

			content, err := os.ReadFile(filepath.Join(workspace, "droplets", "some-app.tar.gz"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-droplet-contents"))

			buildCache, err := os.Open(filepath.Join(workspace, "build-cache", "some-app.tar.gz"))
			Expect(err).NotTo(HaveOccurred())

			err = vacation.NewTarGzipArchive(buildCache).Decompress(filepath.Join(workspace, "build-cache-output"))
			Expect(err).NotTo(HaveOccurred())

			content, err = os.ReadFile(filepath.Join(workspace, "build-cache-output", "tmp", "cache", "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-cache-contents"))
		})

		context("when the container exits with a non-zero status", func() {
			it.Before(func() {
				containerWaitOKBodyChannel := make(chan container.ContainerWaitOKBody)
				go func() {
					containerWaitOKBodyChannel <- container.ContainerWaitOKBody{
						StatusCode: 223,
					}
					close(containerWaitOKBodyChannel)
				}()

				client.ContainerWaitCall.Returns.ContainerWaitOKBodyChannel = containerWaitOKBodyChannel
			})

			it("returns an error", func() {
				ctx := gocontext.Background()
				logs := bytes.NewBuffer(nil)

				_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
				Expect(err).To(MatchError("App staging failed: container exited with non-zero status code (223)"))

				Expect(client.ContainerStartCall.Receives.ContainerID).To(Equal("some-container-id"))

				Expect(client.ContainerWaitCall.Receives.ContainerID).To(Equal("some-container-id"))
				Expect(client.ContainerWaitCall.Receives.Condition).To(Equal(container.WaitConditionNotRunning))

				Expect(client.ContainerLogsCall.Receives.Container).To(Equal("some-container-id"))
				Expect(client.ContainerLogsCall.Receives.Options).To(Equal(types.ContainerLogsOptions{
					ShowStdout: true,
					ShowStderr: true,
				}))

				Expect(client.ContainerRemoveCall.Receives.ContainerID).To(Equal("some-container-id"))
				Expect(client.ContainerRemoveCall.Receives.Options).To(Equal(types.ContainerRemoveOptions{Force: true}))

				Expect(copyFromContainerInvocations).To(HaveLen(0))

				Expect(logs).To(ContainLines("Fetching container logs..."))

				Expect(filepath.Join(workspace, "droplets", "some-app.tar.gz")).NotTo(BeAnExistingFile())
			})

			context("failure cases", func() {
				context("when the container cannot be removed", func() {
					it.Before(func() {
						client.ContainerRemoveCall.Returns.Error = errors.New("could not remove container")
					})

					it("returns an error", func() {
						ctx := gocontext.Background()
						logs := bytes.NewBuffer(nil)

						_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
						Expect(err).To(MatchError("failed to remove container: could not remove container"))
					})
				})
			})
		})

		context("failure cases", func() {
			context("when the container cannot be started", func() {
				it.Before(func() {
					client.ContainerStartCall.Returns.Error = errors.New("could not start container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to start container: could not start container"))
				})
			})

			context("when the container cannot be waited on", func() {
				it.Before(func() {
					errChan := make(chan error)
					waitChan := make(chan container.ContainerWaitOKBody)
					go func() {
						errChan <- errors.New("could not wait on container")
						close(errChan)
						close(waitChan)
					}()

					client.ContainerWaitCall.Returns.ErrorChannel = errChan
					client.ContainerWaitCall.Returns.ContainerWaitOKBodyChannel = waitChan
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to wait on container: could not wait on container"))
				})
			})

			context("when the container logs cannot be fetched", func() {
				it.Before(func() {
					client.ContainerLogsCall.Returns.Error = errors.New("could not fetch container logs")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to fetch container logs: could not fetch container logs"))
				})
			})

			context("when the container logs cannot be copied", func() {
				it.Before(func() {
					client.ContainerLogsCall.Returns.ReadCloser = io.NopCloser(iotest.ErrReader(errors.New("could not read logs")))
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to copy container logs: could not read logs"))
				})
			})

			context("when the droplet cannot be copied from the container", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						switch srcPath {
						case "/tmp/droplet":
							return nil, types.ContainerPathStat{}, errors.New("could not copy droplet")
						}

						return nil, types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to copy droplet from container: could not copy droplet"))
				})
			})

			context("when the droplets directory cannot be created", func() {
				it.Before(func() {
					Expect(os.Chmod(workspace, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to create droplets directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the droplet tarball is malformed", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						switch srcPath {
						case "/tmp/droplet":
							return io.NopCloser(iotest.ErrReader(errors.New("could not read tarball"))), types.ContainerPathStat{}, nil
						}

						return nil, types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to retrieve droplet from tarball: could not read tarball"))
				})
			})

			context("when the build cache cannot be copied from the container", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						buffer := bytes.NewBuffer(nil)
						switch srcPath {
						case "/tmp/droplet":
							if err := generateDroplet(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/output-cache":
							return nil, types.ContainerPathStat{}, errors.New("could not copy output-cache")
						}

						return io.NopCloser(buffer), types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to copy build cache from container: could not copy output-cache"))
				})
			})

			context("when the build cache tarball is malformed", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						buffer := bytes.NewBuffer(nil)

						switch srcPath {
						case "/tmp/droplet":
							if err := generateDroplet(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/output-cache":
							return io.NopCloser(iotest.ErrReader(errors.New("could not read tarball"))), types.ContainerPathStat{}, nil
						}

						return io.NopCloser(buffer), types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to retrieve build cache from tarball: could not read tarball"))
				})
			})

			context("when the build cache internal tarball is malformed", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						buffer := bytes.NewBuffer(nil)

						switch srcPath {
						case "/tmp/droplet":
							if err := generateDroplet(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/output-cache":
							tw := tar.NewWriter(buffer)
							defer tw.Close()
							err := tw.WriteHeader(&tar.Header{Name: "output-cache", Mode: 0600, Size: 20})
							if err != nil {
								return nil, types.ContainerPathStat{}, err
							}

							_, err = tw.Write([]byte("not-a-valid-tarball"))
							if err != nil {
								return nil, types.ContainerPathStat{}, err
							}
						}

						return io.NopCloser(buffer), types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to decompress build cache:")))
					Expect(err).To(MatchError(ContainSubstring("invalid header")))
				})
			})

			context("when the result cannot be copied from the container", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						buffer := bytes.NewBuffer(nil)
						switch srcPath {
						case "/tmp/droplet":
							if err := generateDroplet(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/output-cache":
							if err := generateBuildCache(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/result.json":
							return nil, types.ContainerPathStat{}, errors.New("could not copy result.json")
						}

						return io.NopCloser(buffer), types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to copy result.json from container: could not copy result.json"))
				})
			})

			context("when the result tarball is malformed", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						buffer := bytes.NewBuffer(nil)
						switch srcPath {
						case "/tmp/droplet":
							if err := generateDroplet(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/output-cache":
							if err := generateBuildCache(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/result.json":
							return io.NopCloser(iotest.ErrReader(errors.New("could not read tarball"))), types.ContainerPathStat{}, nil
						}

						return io.NopCloser(buffer), types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to retrieve result.json from tarball: could not read tarball"))
				})
			})

			context("when the result json is malformed", func() {
				it.Before(func() {
					client.CopyFromContainerCall.Stub = func(ctx gocontext.Context, containerID, srcPath string) (io.ReadCloser, types.ContainerPathStat, error) {
						buffer := bytes.NewBuffer(nil)

						switch srcPath {
						case "/tmp/droplet":
							if err := generateDroplet(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/output-cache":
							if err := generateBuildCache(buffer); err != nil {
								return nil, types.ContainerPathStat{}, err
							}

						case "/tmp/result.json":
							if err := generateResultJSON(buffer, "%%%"); err != nil {
								return nil, types.ContainerPathStat{}, err
							}
						}

						return io.NopCloser(buffer), types.ContainerPathStat{}, nil
					}
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError(ContainSubstring("failed to parse result.json:")))
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			context("when the container cannot be removed", func() {
				it.Before(func() {
					client.ContainerRemoveCall.Returns.Error = errors.New("could not remove container")
				})

				it("returns an error", func() {
					ctx := gocontext.Background()
					logs := bytes.NewBuffer(nil)

					_, err := stage.Run(ctx, logs, "some-container-id", "some-app")
					Expect(err).To(MatchError("failed to remove container: could not remove container"))
				})
			})
		})
	})
}

func generateDroplet(buffer io.Writer) error {
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	err := tw.WriteHeader(&tar.Header{Name: "droplet", Mode: 0600, Size: 21})
	if err != nil {
		return err
	}

	_, err = tw.Write([]byte("some-droplet-contents"))
	if err != nil {
		return err
	}

	return nil
}

func generateBuildCache(buffer io.Writer) error {
	cache := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(cache)
	ttw := tar.NewWriter(gw)

	err := ttw.WriteHeader(&tar.Header{Name: "some-file", Mode: 0600, Size: 19})
	if err != nil {
		return err
	}

	_, err = ttw.Write([]byte("some-cache-contents"))
	if err != nil {
		return err
	}

	if err := ttw.Close(); err != nil {
		return err
	}

	if err := gw.Close(); err != nil {
		return err
	}

	tw := tar.NewWriter(buffer)
	defer tw.Close()
	err = tw.WriteHeader(&tar.Header{Name: "output-cache", Mode: 0600, Size: int64(cache.Len())})
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, cache)
	if err != nil {
		return err
	}

	return nil
}

func generateResultJSON(buffer io.Writer, result string) error {
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	r := []byte(result)
	err := tw.WriteHeader(&tar.Header{Name: "result.json", Mode: 0600, Size: int64(len(r))})
	if err != nil {
		return err
	}

	_, err = tw.Write(r)
	if err != nil {
		return err
	}

	return nil
}

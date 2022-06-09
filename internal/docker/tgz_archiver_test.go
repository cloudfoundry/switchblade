package docker_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/switchblade/internal/docker"
	"github.com/paketo-buildpacks/packit/v2/vacation"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testTGZArchiver(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		archiver              docker.TGZArchiver
		tmpDir, input, output string
	)

	it.Before(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())

		input = filepath.Join(tmpDir, "input")
		output = filepath.Join(tmpDir, "output", "output.tgz")

		err = os.MkdirAll(filepath.Join(input, "some-dir"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(input, "some-file"), []byte("some-content"), 0400)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(input, "some-dir", "other-file"), []byte("other-content"), 0600)
		Expect(err).NotTo(HaveOccurred())

		err = os.Symlink(filepath.Join(".", "other-file"), filepath.Join(input, "some-dir", "some-link"))
		Expect(err).NotTo(HaveOccurred())

		archiver = docker.NewTGZArchiver()
	})

	it.After(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	context("when the path is a directory", func() {
		it("creates an archive of the given path", func() {
			err := archiver.Compress(input, output)
			Expect(err).NotTo(HaveOccurred())

			testOutput := filepath.Join(tmpDir, "test-output")
			Expect(os.Mkdir(testOutput, os.ModePerm)).To(Succeed())

			file, err := os.Open(output)
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()

			err = vacation.NewGzipArchive(file).Decompress(testOutput)
			Expect(err).NotTo(HaveOccurred())

			files, err := filepath.Glob(filepath.Join(testOutput, "*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(ConsistOf([]string{
				filepath.Join(testOutput, "some-dir"),
				filepath.Join(testOutput, "some-file"),
			}))

			content, err := os.ReadFile(filepath.Join(testOutput, "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-content"))

			info, err := os.Stat(filepath.Join(testOutput, "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode()).To(Equal(fs.FileMode(0400)))

			content, err = os.ReadFile(filepath.Join(testOutput, "some-dir", "other-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("other-content"))

			info, err = os.Stat(filepath.Join(testOutput, "some-dir", "other-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode()).To(Equal(fs.FileMode(0600)))

			link, err := os.Readlink(filepath.Join(testOutput, "some-dir", "some-link"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal("other-file"))
		})

		context("when given a prefix", func() {
			it("includes the prefix on the file paths in the archive", func() {
				err := archiver.WithPrefix("/some/path").Compress(input, output)
				Expect(err).NotTo(HaveOccurred())

				testOutput := filepath.Join(tmpDir, "test-output")
				Expect(os.Mkdir(testOutput, os.ModePerm)).To(Succeed())

				file, err := os.Open(output)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				err = vacation.NewGzipArchive(file).Decompress(testOutput)
				Expect(err).NotTo(HaveOccurred())

				files, err := filepath.Glob(filepath.Join(testOutput, "some", "path", "*"))
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(ConsistOf([]string{
					filepath.Join(testOutput, "some", "path", "some-dir"),
					filepath.Join(testOutput, "some", "path", "some-file"),
				}))

				content, err := os.ReadFile(filepath.Join(testOutput, "some", "path", "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-content"))

				info, err := os.Stat(filepath.Join(testOutput, "some", "path", "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Mode()).To(Equal(fs.FileMode(0400)))

				content, err = os.ReadFile(filepath.Join(testOutput, "some", "path", "some-dir", "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("other-content"))

				info, err = os.Stat(filepath.Join(testOutput, "some", "path", "some-dir", "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Mode()).To(Equal(fs.FileMode(0600)))

				link, err := os.Readlink(filepath.Join(testOutput, "some", "path", "some-dir", "some-link"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal("other-file"))
			})
		})

		context("failure cases", func() {
			context("when a file in the input cannot be opened", func() {
				it.Before(func() {
					Expect(os.Chmod(filepath.Join(input, "some-file"), 0000)).To(Succeed())
				})

				it("returns an error", func() {
					err := archiver.Compress(input, output)
					Expect(err).To(MatchError(ContainSubstring("failed to open file:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})
		})
	})

	context("when the path is a tarball", func() {
		it.Before(func() {
			file, err := os.CreateTemp(tmpDir, "input-archive")
			Expect(err).NotTo(HaveOccurred())

			gw := gzip.NewWriter(file)
			defer gw.Close()

			tw := tar.NewWriter(gw)
			defer tw.Close()

			err = filepath.Walk(input, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}

				var link string
				if info.Mode()&fs.ModeSymlink != 0 {
					link, err = os.Readlink(path)
					if err != nil {
						return err
					}

					if !strings.HasPrefix(link, string(filepath.Separator)) {
						link = filepath.Clean(filepath.Join(filepath.Dir(path), link))
					}

					link, err = filepath.Rel(filepath.Dir(path), link)
					if err != nil {
						return err
					}
				}

				hdr, err := tar.FileInfoHeader(info, link)
				if err != nil {
					return err
				}

				hdr.Name, err = filepath.Rel(input, path)
				if err != nil {
					return err
				}

				err = tw.WriteHeader(hdr)
				if err != nil {
					return err
				}

				if info.Mode().IsRegular() {
					fd, err := os.Open(path)
					if err != nil {
						return err
					}
					defer fd.Close()

					_, err = io.Copy(tw, fd)
					if err != nil {
						return err
					}
				}

				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			input = file.Name()
		})

		it("creates an archive of the given path", func() {
			err := archiver.Compress(input, output)
			Expect(err).NotTo(HaveOccurred())

			testOutput := filepath.Join(tmpDir, "test-output")
			Expect(os.Mkdir(testOutput, os.ModePerm)).To(Succeed())

			file, err := os.Open(output)
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()

			err = vacation.NewGzipArchive(file).Decompress(testOutput)
			Expect(err).NotTo(HaveOccurred())

			files, err := filepath.Glob(filepath.Join(testOutput, "*"))
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(ConsistOf([]string{
				filepath.Join(testOutput, "some-dir"),
				filepath.Join(testOutput, "some-file"),
			}))

			content, err := os.ReadFile(filepath.Join(testOutput, "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("some-content"))

			info, err := os.Stat(filepath.Join(testOutput, "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode()).To(Equal(fs.FileMode(0400)))

			content, err = os.ReadFile(filepath.Join(testOutput, "some-dir", "other-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("other-content"))

			info, err = os.Stat(filepath.Join(testOutput, "some-dir", "other-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode()).To(Equal(fs.FileMode(0600)))

			link, err := os.Readlink(filepath.Join(testOutput, "some-dir", "some-link"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal("other-file"))
		})

		context("when given a prefix", func() {
			it("includes the prefix on the file paths in the archive", func() {
				err := archiver.WithPrefix("/some/path").Compress(input, output)
				Expect(err).NotTo(HaveOccurred())

				testOutput := filepath.Join(tmpDir, "test-output")
				Expect(os.Mkdir(testOutput, os.ModePerm)).To(Succeed())

				file, err := os.Open(output)
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()

				err = vacation.NewGzipArchive(file).Decompress(testOutput)
				Expect(err).NotTo(HaveOccurred())

				files, err := filepath.Glob(filepath.Join(testOutput, "some", "path", "*"))
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(ConsistOf([]string{
					filepath.Join(testOutput, "some", "path", "some-dir"),
					filepath.Join(testOutput, "some", "path", "some-file"),
				}))

				content, err := os.ReadFile(filepath.Join(testOutput, "some", "path", "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-content"))

				info, err := os.Stat(filepath.Join(testOutput, "some", "path", "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Mode()).To(Equal(fs.FileMode(0400)))

				content, err = os.ReadFile(filepath.Join(testOutput, "some", "path", "some-dir", "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("other-content"))

				info, err = os.Stat(filepath.Join(testOutput, "some", "path", "some-dir", "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Mode()).To(Equal(fs.FileMode(0600)))

				link, err := os.Readlink(filepath.Join(testOutput, "some", "path", "some-dir", "some-link"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal("other-file"))
			})
		})

		context("failure cases", func() {
			context("when the input file cannot be opened", func() {
				it.Before(func() {
					Expect(os.Chmod(input, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					err := archiver.Compress(input, output)
					Expect(err).To(MatchError(ContainSubstring("failed to open file:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})
		})
	})
}

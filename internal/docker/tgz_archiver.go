package docker

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type TGZArchiver struct {
	prefix string
}

func NewTGZArchiver() TGZArchiver {
	return TGZArchiver{}
}

func (a TGZArchiver) WithPrefix(prefix string) Archiver {
	a.prefix = prefix
	return a
}

func (a TGZArchiver) Compress(input, output string) error {
	err := os.MkdirAll(filepath.Dir(output), os.ModePerm)
	if err != nil {
		panic(err)
	}

	file, err := os.Create(output)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(input, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}

		var link string
		if info.Mode()&os.ModeType != 0 && !info.IsDir() {
			link, err = os.Readlink(path)
			if err != nil {
				panic(err)
			}

			if !strings.HasPrefix(link, string(filepath.Separator)) {
				link = filepath.Clean(filepath.Join(filepath.Dir(path), link))
			}

			link, err = filepath.Rel(filepath.Dir(path), link)
			if err != nil {
				panic(err)
			}
		}

		rel, err := filepath.Rel(input, path)
		if err != nil {
			panic(err)
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			panic(err)
		}

		header.Name = filepath.Join(a.prefix, rel)
		header.Uid = 2000
		header.Gid = 2000
		header.Uname = "vcap"
		header.Gname = "vcap"

		err = tw.WriteHeader(header)
		if err != nil {
			panic(err)
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			_, err = io.Copy(tw, f)
			if err != nil {
				panic(err)
			}
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	return nil
}

package service

import (
	"archive/zip"
	"bytes"
	"io"
)

func unzipFirstFile(data []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(r.File) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	f, err := r.File[0].Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

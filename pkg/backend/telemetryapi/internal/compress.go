package internal

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

// Uncompress un-gzips the given input.
func Uncompress(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	gz, err := gzip.NewReader(buf)
	if nil != err {
		return nil, err
	}
	defer gz.Close()
	return ioutil.ReadAll(gz)
}

// Compress gzips the given input.
func Compress(b []byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(b)
	w.Close()

	if nil != err {
		return nil, err
	}

	return &buf, nil
}

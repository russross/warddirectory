package main

import (
	"archive/zip"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"net/url"
	"path/filepath"
	"strings"
)

var dataFiles map[string][]byte = make(map[string][]byte)

type bytebuf []byte

func (b bytebuf) ReadAt(p []byte, off int64) (n int, err error) {
	slice := []byte(b)
	if off >= int64(len(slice)) {
		return 0, errors.New("End of file")
	}
	n = copy(p, slice[off:])
	return n, nil
}

// this gets called by init in font.go
func loadDataFiles() {
	// decode the base64
	for base64ZipData[len(base64ZipData)-1] == '\n' {
		base64ZipData = base64ZipData[:len(base64ZipData)-1]
	}
	zipData, err := base64.StdEncoding.DecodeString(base64ZipData)
	if err != nil {
		log.Fatalf("decoding base64: %v", err)
	}
	base64ZipData = ""

	// open the zip file
	z, err := zip.NewReader(bytebuf(zipData), int64(len(zipData)))
	if err != nil {
		log.Fatalf("opening zip file: %v", err)
	}

	// process each file
	for _, elt := range z.File {
		name := unescapeUrl(elt.Name)
		fp, err := elt.Open()
		if err != nil {
			log.Fatalf("opening %s from zip file: %v", name, err)
		}
		data, err := ioutil.ReadAll(fp)
		if err != nil {
			log.Fatalf("reading %s from zip file: %v", name, err)
		}
		fp.Close()

		// save it with its original filename as the key
		key := filepath.Base(name)
		dataFiles[key] = data
	}
}

func unescapeUrl(s string) string {
	parts := strings.Split(s, "/")
	for i, elt := range parts {
		if orig, err := url.QueryUnescape(elt); err == nil {
			parts[i] = orig
		}
	}
	return strings.Join(parts, "/")
}

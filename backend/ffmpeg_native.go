//go:build !embedffmpeg

package main

import "errors"

func hasEmbeddedFFmpeg() bool {
	return false
}

func extractEmbeddedFFmpeg() (string, error) {
	return "", errors.New("ffmpeg not embedded: built without embedffmpeg tag")
}

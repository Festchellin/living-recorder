//go:build embedffmpeg

package main

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed embed/ffmpeg
var ffmpegFiles embed.FS

func embeddedFFmpegName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func embeddedFFprobeName() string {
	if runtime.GOOS == "windows" {
		return "ffprobe.exe"
	}
	return "ffprobe"
}

func hasEmbeddedFFmpeg() bool {
	_, err := ffmpegFiles.ReadFile("embed/ffmpeg/" + embeddedFFmpegName())
	return err == nil
}

func extractEmbeddedFFmpeg() (dir string, err error) {
	dir, err = os.MkdirTemp("", "living-recorder-ffmpeg-*")
	if err != nil {
		return "", err
	}

	for _, name := range []string{embeddedFFmpegName(), embeddedFFprobeName()} {
		data, readErr := ffmpegFiles.ReadFile("embed/ffmpeg/" + name)
		if readErr != nil {
			continue
		}
		path := filepath.Join(dir, name)
		if writeErr := os.WriteFile(path, data, 0755); writeErr != nil {
			os.RemoveAll(dir)
			return "", writeErr
		}
	}

	return dir, nil
}

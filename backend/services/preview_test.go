package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func writeMockBinaries(t *testing.T, dir string) (ffmpeg, ffprobe string) {
	t.Helper()

	ffprobe = filepath.Join(dir, "ffprobe")
	probeScript := `#!/bin/bash
cat <<'EOF'
{"streams":[{"codec_type":"video","width":1920,"height":1080,"avg_frame_rate":"30/1"}]}
EOF`
	if err := os.WriteFile(ffprobe, []byte(probeScript), 0755); err != nil {
		t.Fatal(err)
	}

	ffmpeg = filepath.Join(dir, "ffmpeg")
	ffmpegScript := `#!/bin/bash
# Output test data indefinitely until killed
while true; do
    dd if=/dev/zero bs=1024 count=64 2>/dev/null
    sleep 0.05
done
`
	if err := os.WriteFile(ffmpeg, []byte(ffmpegScript), 0755); err != nil {
		t.Fatal(err)
	}

	return
}

func newTestWSClient(t *testing.T) (*websocket.Conn, *httptest.Server) {
	t.Helper()
	var upgrader = websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Echo server — reads and discards, writes back what it receives
		go func() {
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()
	}))
	t.Cleanup(server.Close)

	url := "ws" + server.URL[4:] // http:// → ws://
	cli, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cli.Close() })

	return cli, server
}

func newTestWSConnRaw(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	var upgrader = websocket.Upgrader{}

	cliCh := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		cliCh <- conn
	}))
	t.Cleanup(server.Close)

	url := "ws" + server.URL[4:]
	cli, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cli.Close() })

	srv := <-cliCh

	return cli, srv
}

func TestPreviewManagerSubscribeUnsubscribe(t *testing.T) {
	dir := t.TempDir()
	ffmpeg, ffprobe := writeMockBinaries(t, dir)

	pm := NewPreviewManager(ffmpeg, ffprobe, "")

	cli, _ := newTestWSClient(t)

	cfg := PreviewConfig{Width: 640, Height: 360, FPS: 10, CRF: 35}
	err := pm.Subscribe(1, "rtsp://example.com/stream", "rtsp", cli, cfg)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Give the mock ffmpeg time to produce output
	time.Sleep(50 * time.Millisecond)

	pm.Unsubscribe(1, cli)
	// Ensure it doesn't panic on double-unsubscribe
	pm.Unsubscribe(1, cli)
}

func TestPreviewManagerMultiSubscriber(t *testing.T) {
	dir := t.TempDir()
	ffmpeg, ffprobe := writeMockBinaries(t, dir)
	pm := NewPreviewManager(ffmpeg, ffprobe, "")

	var clients []*websocket.Conn
	for i := 0; i < 3; i++ {
		cli, _ := newTestWSClient(t)
		clients = append(clients, cli)
	}

	var wg sync.WaitGroup
	for i, cli := range clients {
		wg.Add(1)
		go func(id uint, conn *websocket.Conn) {
			defer wg.Done()
			cfg := PreviewConfig{Width: 640, Height: 360, FPS: 10, CRF: 35}
			err := pm.Subscribe(id, "rtsp://example.com/stream", "rtsp", conn, cfg)
			if err != nil {
				t.Errorf("Subscribe(%d) failed: %v", id, err)
			}
		}(uint(i+1), cli)
	}
	wg.Wait()

	time.Sleep(50 * time.Millisecond)

	for i, cli := range clients {
		pm.Unsubscribe(uint(i+1), cli)
	}
}

func TestPreviewManagerRefCountLifecycle(t *testing.T) {
	dir := t.TempDir()
	ffmpeg, ffprobe := writeMockBinaries(t, dir)

	pm := NewPreviewManager(ffmpeg, ffprobe, "")

	// Two subscribers for the same stream
	cli1, _ := newTestWSClient(t)
	cli2, _ := newTestWSClient(t)

	cfg := PreviewConfig{Width: 640, Height: 360, FPS: 10, CRF: 35}
	if err := pm.Subscribe(1, "rtsp://example.com/stream", "rtsp", cli1, cfg); err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}
	if err := pm.Subscribe(1, "rtsp://example.com/stream", "rtsp", cli2, cfg); err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	// Unsubscribe first — refCount goes to 1, stream still alive
	pm.Unsubscribe(1, cli1)

	// Stream should still be in map
	pm.mu.RLock()
	_, exists := pm.streams[1]
	pm.mu.RUnlock()
	if !exists {
		t.Fatal("expected stream to still exist after one unsubscribe")
	}

	// Unsubscribe second — refCount goes to 0, stream should be cleaned up
	pm.Unsubscribe(1, cli2)

	pm.mu.RLock()
	_, exists = pm.streams[1]
	pm.mu.RUnlock()
	if exists {
		t.Fatal("expected stream to be removed after all unsubscribes")
	}
}

func TestPreviewManagerConcurrentSubscribeUnsubscribe(t *testing.T) {
	dir := t.TempDir()
	ffmpeg, ffprobe := writeMockBinaries(t, dir)
	pm := NewPreviewManager(ffmpeg, ffprobe, "")

	var clients []*websocket.Conn
	for i := 0; i < 10; i++ {
		cli, _ := newTestWSClient(t)
		clients = append(clients, cli)
	}

	var subscribeErr int32
	var wg sync.WaitGroup

	// All subscribe to the same stream concurrently
	for _, cli := range clients {
		wg.Add(1)
		go func(conn *websocket.Conn) {
			defer wg.Done()
			cfg := PreviewConfig{Width: 640, Height: 360, FPS: 10, CRF: 35}
			if err := pm.Subscribe(1, "rtsp://example.com/stream", "rtsp", conn, cfg); err != nil {
				atomic.AddInt32(&subscribeErr, 1)
			}
		}(cli)
	}
	wg.Wait()

	if subscribeErr > 0 {
		t.Fatalf("%d subscribes failed", subscribeErr)
	}

	time.Sleep(20 * time.Millisecond)

	// All unsubscribe concurrently
	var unsubWg sync.WaitGroup
	for _, cli := range clients {
		unsubWg.Add(1)
		go func(conn *websocket.Conn) {
			defer unsubWg.Done()
			pm.Unsubscribe(1, conn)
		}(cli)
	}
	unsubWg.Wait()

	pm.mu.RLock()
	_, exists := pm.streams[1]
	pm.mu.RUnlock()
	if exists {
		t.Fatal("expected stream to be removed after all concurrent unsubscribes")
	}
}

func TestPreviewManagerStopAll(t *testing.T) {
	dir := t.TempDir()
	ffmpeg, ffprobe := writeMockBinaries(t, dir)
	pm := NewPreviewManager(ffmpeg, ffprobe, "")

	var clients []*websocket.Conn
	for i := 0; i < 3; i++ {
		cli, _ := newTestWSClient(t)
		clients = append(clients, cli)
	}

	for i, cli := range clients {
		cfg := PreviewConfig{Width: 640, Height: 360, FPS: 10, CRF: 35}
		if err := pm.Subscribe(uint(i+1), "rtsp://example.com/stream", "rtsp", cli, cfg); err != nil {
			t.Fatalf("Subscribe %d failed: %v", i+1, err)
		}
	}

	time.Sleep(20 * time.Millisecond)

	pm.StopAll()

	pm.mu.RLock()
	count := len(pm.streams)
	pm.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected 0 streams after StopAll, got %d", count)
	}
}

func TestPreviewManagerProbeCache(t *testing.T) {
	dir := t.TempDir()
	_, ffprobe := writeMockBinaries(t, dir)
	pm := NewPreviewManager("ffmpeg", ffprobe, "")

	info := pm.probeSource("rtsp://example.com/stream", "rtsp")
	if info.width != 1920 {
		t.Fatalf("expected width 1920, got %d", info.width)
	}
	if info.height != 1080 {
		t.Fatalf("expected height 1080, got %d", info.height)
	}
	if info.fps != 30 {
		t.Fatalf("expected fps 30, got %d", info.fps)
	}

	// Second call should be cached
	info2 := pm.probeSource("rtsp://example.com/stream", "rtsp")
	if info2.width != 1920 {
		t.Fatalf("expected cached width 1920, got %d", info2.width)
	}
}

func TestParseFPS(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"30/1", 30},
		{"60/1", 60},
		{"25/1", 25},
		{"30000/1001", 29},
		{"0/1", 0},
		{"invalid", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseFPS(tt.input)
		if got != tt.want {
			t.Errorf("parseFPS(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

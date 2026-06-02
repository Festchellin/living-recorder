package main

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"living-recorder/backend/config"
	"living-recorder/backend/database"
	"living-recorder/backend/models"
	"living-recorder/backend/routes"
	"living-recorder/backend/services"

	"github.com/gin-gonic/gin"
)

//go:embed all:embed/dist
var staticFiles embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if hasEmbeddedFFmpeg() {
		dir, err := extractEmbeddedFFmpeg()
		if err != nil {
			log.Fatalf("Failed to extract embedded ffmpeg: %v", err)
		}
		os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		cfg.FFmpeg.Path = filepath.Join(dir, "ffmpeg")
		if runtime.GOOS == "windows" {
			cfg.FFmpeg.Path += ".exe"
		}
		log.Printf("Extracted embedded ffmpeg to %s", dir)
	}

	db, err := database.Init(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	var store services.StorageBackend
	switch cfg.Storage.Default {
	case "s3":
		s3store, err := services.NewS3Storage(cfg.Storage.S3)
		if err != nil {
			log.Fatalf("Failed to init S3 storage: %v", err)
		}
		store = s3store
	default:
		store = services.NewLocalStorage(cfg.Storage.Local)
	}

	logWriter := services.NewLogWriter(db)
	defer logWriter.Stop()
	recorder := services.NewRecorderService(db, &cfg.Recorder, cfg.FFmpeg.Path, store)
	recorder.SetStore(cfg.Storage.Default, store)
	scheduler := services.NewSchedulerService(db, recorder)
	monitor := services.NewMonitorService(db, recorder, cfg.Recorder)

	recorder.OnStatusChange(monitor.NotifyStreamChange)
	recorder.ResetStaleStatuses()
	logWriter.Info(models.EventSystemStartup, "系统启动 — 端口=%s 录制目录=%s", cfg.Server.Port, cfg.Recorder.StorageLocalPath)

	previewMgr := services.NewPreviewManager(cfg.FFmpeg.Path, "", recorder.GetHardwareEncoder())

	scheduler.Start()
	monitor.Start()

	gin.SetMode(cfg.Server.Mode)
	r := routes.Setup(db, cfg, recorder, scheduler, monitor, previewMgr)

	staticFS, _ := fs.Sub(staticFiles, "embed/dist")
	readStatic := func(name string) ([]byte, error) {
		f, err := staticFS.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	r.GET("/", func(c *gin.Context) {
		data, err := readStatic("index.html")
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	r.GET("/assets/*filepath", func(c *gin.Context) {
		fpath := "assets" + c.Param("filepath")
		data, err := readStatic(fpath)
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		ctype := mime.TypeByExtension(filepath.Ext(c.Param("filepath")))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		c.Data(http.StatusOK, ctype, data)
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"code": 1, "message": "not found"})
			return
		}
		data, err := readStatic("index.html")
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	recorder.StartHealthCheck(context.Background())

	addr := ":" + cfg.Server.Port
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

package main

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"living-recorder/backend/config"
	"living-recorder/backend/database"
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

	recorder := services.NewRecorderService(db, cfg.Recorder, cfg.FFmpeg.Path, store)
	scheduler := services.NewSchedulerService(db, recorder)
	monitor := services.NewMonitorService(db, recorder, cfg.Recorder)

	scheduler.Start()
	monitor.Start()

	gin.SetMode(cfg.Server.Mode)
	r := routes.Setup(db, cfg, recorder, scheduler, monitor)

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

	addr := ":" + cfg.Server.Port
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

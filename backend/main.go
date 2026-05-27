package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

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
	r.GET("/", func(c *gin.Context) {
		http.FileServer(http.FS(staticFS)).ServeHTTP(c.Writer, c.Request)
	})
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 5 && c.Request.URL.Path[:5] == "/api/" {
			c.JSON(404, gin.H{"code": 1, "message": "not found"})
			return
		}
		c.Request.URL.Path = "/index.html"
		http.FileServer(http.FS(staticFS)).ServeHTTP(c.Writer, c.Request)
	})

	addr := ":" + cfg.Server.Port
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

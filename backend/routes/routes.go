package routes

import (
	"living-recorder/backend/config"
	"living-recorder/backend/handlers"
	"living-recorder/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, cfg *config.Config, recorder *services.RecorderService, scheduler *services.SchedulerService, monitor *services.MonitorService, previewMgr *services.PreviewManager) *gin.Engine {
	r := gin.New()
	r.RedirectFixedPath = false
	r.RedirectTrailingSlash = false
	r.Use(gin.Logger(), gin.Recovery())

	streamHandler := handlers.NewStreamHandler(db, recorder, previewMgr)
	taskHandler := handlers.NewTaskHandler(db, scheduler)
	statusHandler := handlers.NewStatusHandler(db, cfg, recorder)
	groupHandler := handlers.NewGroupHandler(db)
	wsHandler := handlers.NewWSHandler(monitor)

	api := r.Group("/api")
	{
		api.GET("/streams", streamHandler.List)
		api.POST("/streams", streamHandler.Create)
		api.GET("/streams/:id", streamHandler.Get)
		api.PUT("/streams/:id", streamHandler.Update)
		api.DELETE("/streams/:id", streamHandler.Delete)
		api.POST("/streams/:id/start", streamHandler.Start)
		api.POST("/streams/:id/stop", streamHandler.Stop)
		api.GET("/streams/:id/logs", streamHandler.Logs)
		api.GET("/streams/:id/probe", streamHandler.Probe)
		api.GET("/streams/:id/probe-info", streamHandler.ProbeInfo)
		api.POST("/streams/start-all", streamHandler.StartAll)
		api.POST("/streams/stop-all", streamHandler.StopAll)
		api.POST("/streams/export", streamHandler.Export)
		api.POST("/streams/import", streamHandler.Import)
		api.GET("/preview/:id/ws", streamHandler.PreviewWS)

		api.GET("/tasks", taskHandler.List)
		api.POST("/tasks", taskHandler.Create)
		api.PUT("/tasks/:id", taskHandler.Update)
		api.DELETE("/tasks/:id", taskHandler.Delete)

		api.GET("/status", statusHandler.GetStatus)
		api.GET("/config", statusHandler.GetConfig)
		api.PUT("/config", statusHandler.UpdateConfig)
		api.POST("/config/test-s3", statusHandler.TestS3Connection)

		api.GET("/groups", groupHandler.List)
		api.POST("/groups", groupHandler.Create)
		api.PUT("/groups/reorder", groupHandler.Reorder)
		api.PUT("/groups/:id", groupHandler.Update)
		api.DELETE("/groups/:id", groupHandler.Delete)

		api.GET("/logs/recent", streamHandler.RecentLogs)

		api.GET("/ws", wsHandler.Handle)
	}

	return r
}

package routes

import (
	"living-recorder/backend/config"
	"living-recorder/backend/handlers"
	"living-recorder/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, cfg *config.Config, recorder *services.RecorderService, scheduler *services.SchedulerService, monitor *services.MonitorService) *gin.Engine {
	r := gin.New()
	r.RedirectFixedPath = false
	r.RedirectTrailingSlash = false
	r.Use(gin.Logger(), gin.Recovery())

	streamHandler := handlers.NewStreamHandler(db, recorder)
	taskHandler := handlers.NewTaskHandler(db, scheduler)
	statusHandler := handlers.NewStatusHandler(db, cfg, recorder)
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
		api.POST("/streams/start-all", streamHandler.StartAll)
		api.POST("/streams/stop-all", streamHandler.StopAll)

		api.GET("/tasks", taskHandler.List)
		api.POST("/tasks", taskHandler.Create)
		api.PUT("/tasks/:id", taskHandler.Update)
		api.DELETE("/tasks/:id", taskHandler.Delete)

		api.GET("/status", statusHandler.GetStatus)
		api.GET("/config", statusHandler.GetConfig)
		api.PUT("/config", statusHandler.UpdateConfig)

		api.GET("/ws", wsHandler.Handle)
	}

	return r
}

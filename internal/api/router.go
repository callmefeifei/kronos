package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/auth"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
	"gorm.io/gorm"
)

// serverStartTime records when the router was created, used by the status endpoint.
var serverStartTime = time.Now()

// NewRouter creates and configures the Gin router with all API endpoints.
func NewRouter(
	cfg *config.Config,
	db *gorm.DB,
	userStore *store.UserStore,
	taskStore *store.TaskStore,
	taskRunStore *store.TaskRunStore,
	notificationStore *store.NotificationStore,
	sched *scheduler.Scheduler,
	distFS fs.FS,
) *gin.Engine {
	// Set Gin mode from config.
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// CORS middleware for frontend dev.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Health check.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Server status (public, used by CLI `kronos status`).
	r.GET("/api/v1/status", func(c *gin.Context) {
		uptime := time.Since(serverStartTime)
		hours := int(uptime.Hours())
		minutes := int(uptime.Minutes()) % 60
		seconds := int(uptime.Seconds()) % 60
		uptimeStr := fmt.Sprintf("%dd %dh %dm %ds", hours/24, hours%24, minutes, seconds)
		Success(c, gin.H{
			"version":     "dev",
			"uptime":      uptimeStr,
			"server_time": time.Now().Format(time.RFC3339),
			"go_version":  runtime.Version(),
		})
	})

	// Create handlers.
	authHandler := NewAuthHandler(userStore, cfg.Auth)
	userHandler := NewUserHandler(userStore)
	taskHandler := NewTaskHandler(taskStore, sched)
	runHandler := NewRunHandler(taskStore, taskRunStore)
	notifHandler := NewNotificationHandler(notificationStore)
	dashHandler := NewDashboardHandler(db)

	v1 := r.Group("/api/v1")

	// --- Public routes ---
	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)
	}

	// --- Protected routes (require authentication) ---
	protected := v1.Group("")
	protected.Use(auth.AuthRequired(cfg.Auth, userStore))

	// Users (admin only).
	users := protected.Group("/users")
	users.Use(auth.AdminRequired())
	{
		users.GET("", userHandler.List)
		users.POST("", userHandler.Create)
		users.PUT("/:id", userHandler.Update)
		users.DELETE("/:id", userHandler.Delete)
	}

	// Tasks.
	tasks := protected.Group("/tasks")
	{
		tasks.GET("", taskHandler.List)
		tasks.POST("", taskHandler.Create)
		tasks.GET("/:id", taskHandler.Get)
		tasks.PUT("/:id", taskHandler.Update)
		tasks.DELETE("/:id", taskHandler.Delete)
		tasks.POST("/:id/run", taskHandler.Run)
		tasks.POST("/:id/enable", taskHandler.Enable)
		tasks.POST("/:id/disable", taskHandler.Disable)
		tasks.GET("/:id/runs", runHandler.ListByTask)
	}

	// Runs.
	runs := protected.Group("/runs")
	{
		runs.GET("/:id", runHandler.Get)
		runs.GET("/:id/output", runHandler.Output)
	}

	// Dashboard.
	dashboard := protected.Group("/dashboard")
	{
		dashboard.GET("/stats", dashHandler.Stats)
		dashboard.GET("/timeline", dashHandler.Timeline)
	}

	// Notifications.
	notifications := protected.Group("/notifications")
	{
		notifications.GET("", notifHandler.List)
	}

	// Serve embedded Vue SPA for non-API routes.
	RegisterSPA(r, distFS)

	return r
}

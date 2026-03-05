package api

import (
	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	r.Use(CORS())

	api := r.Group("/api")
	api.Use(APIKeyAuth())
	{
		// User management
		users := api.Group("/users")
		{
			users.POST("", CreateUser)
			users.GET("", ListUsers)
			users.GET("/:id", GetUser)
			users.PUT("/:id", UpdateUser)
			users.DELETE("/:id", DeleteUser)
			users.POST("/:id/enable", EnableUser)
			users.POST("/:id/disable", DisableUser)
			users.POST("/:id/reset-traffic", ResetTraffic)
			users.GET("/:id/config", GetUserConfig)
			users.GET("/:id/link", GetUserLink)
		}

		// Test users
		api.POST("/test-users", CreateTestUser)

		// Server management
		server := api.Group("/server")
		{
			server.GET("/status", GetServerStatus)
			server.POST("/start", StartServer)
			server.POST("/stop", StopServer)
			server.POST("/restart", RestartServer)
			server.GET("/config", GetServerConfig)
			server.PUT("/config", UpdateServerConfig)
		}

		// Connections
		api.GET("/connections", GetActiveConnections)
		api.POST("/connections/:username/kick", KickUser)
	}
}

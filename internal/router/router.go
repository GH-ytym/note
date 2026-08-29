package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"note/internal/handler"

	"github.com/gin-gonic/gin"
)

func New(h *handler.Handler) *gin.Engine {
	return NewWithWeb(h, "")
}

// NewWithWeb creates the API router and optionally serves a built React app.
// webDir is empty during normal API development and points to web/dist in Electron.
func NewWithWeb(h *handler.Handler, webDir string) *gin.Engine {
	r := gin.Default()
	registerAPI(r, h)
	registerAPI(r.Group("/api"), h)

	if webDir != "" {
		indexPath := filepath.Join(webDir, "index.html")
		r.GET("/", func(c *gin.Context) {
			c.File(indexPath)
		})
		r.Static("/assets", filepath.Join(webDir, "assets"))
		r.NoRoute(func(c *gin.Context) {
			if c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.File(indexPath)
		})
	}

	return r
}

func registerAPI(r gin.IRouter, h *handler.Handler) {
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	r.GET("/calendar", h.GetCalendar)

	todos := r.Group("/todos")
	{
		todos.POST("", h.CreateTodo)
		todos.GET("", h.ListTodos)
		todos.GET("/:id", h.GetTodo)
		todos.PATCH("/:id", h.PatchTodo)
		todos.PATCH(
			"/:id/occurrences/:date",
			h.PatchOccurrenceDone,
		)
		todos.DELETE("/:id", h.DeleteTodo)
	}
}

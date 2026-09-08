package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"note/internal/handler"

	"github.com/gin-gonic/gin"
)

func New(th *handler.TodoHandler, eh *handler.EventHandler) *gin.Engine {
	return NewWithWeb(th, eh, "")
}

// NewWithWeb creates the API router and optionally serves a built React app.
// webDir is empty during normal API development and points to web/dist in Electron.
func NewWithWeb(th *handler.TodoHandler, eh *handler.EventHandler, webDir string) *gin.Engine {
	r := gin.Default()
	registerAPI(r, th, eh)
	registerAPI(r.Group("/api"), th, eh)

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

func registerAPI(r gin.IRouter, th *handler.TodoHandler, eh *handler.EventHandler) {
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	r.GET("/calendar", th.GetCalendar)

	todos := r.Group("/todos")
	{
		todos.POST("", th.CreateTodo)
		todos.GET("", th.ListTodos)
		todos.GET("/:id", th.GetTodo)
		todos.PATCH("/:id", th.PatchTodo)
		todos.PATCH(
			"/:id/occurrences/:date",
			th.PatchOccurrenceDone,
		)
		todos.DELETE("/:id", th.DeleteTodo)
	}

	events := r.Group("/events")
	{
		events.POST("", eh.CreateEvent)
	}
}

package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"note/internal/handler"
	"note/internal/search"

	"github.com/gin-gonic/gin"
)

// NewWithWeb creates the API router and optionally serves a built React app.
// webDir is empty during normal API development and points to web/dist in Electron.
func NewWithWeb(th *handler.TodoHandler, eh *handler.EventHandler, ch *handler.CalendarHandler, sh *search.SearchHandler, webDir string) *gin.Engine {
	r := gin.Default()
	registerAPI(r, th, eh, ch, sh)
	registerAPI(r.Group("/api"), th, eh, ch, sh)

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

func registerAPI(r gin.IRouter, th *handler.TodoHandler, eh *handler.EventHandler, ch *handler.CalendarHandler, sh *search.SearchHandler) {
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	r.GET("/calendar", ch.GetCalendar)
	r.GET("/search/todos", sh.SearchTodos)
	r.GET("/search/events", sh.SearchEvents)
	r.GET("/search/all", sh.SearchAll)

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
		events.GET("/:id", eh.GetEvent)
		events.PATCH("/:id", eh.PatchEvent)
	}

}

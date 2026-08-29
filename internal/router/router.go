package router

import (
	"net/http"

	"note/internal/handler"

	"github.com/gin-gonic/gin"
)

func New(h *handler.Handler) *gin.Engine {
	r := gin.Default()

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

	return r
}

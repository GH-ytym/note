package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseTodoID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid todo id",
		})
		return 0, false
	}

	return uint(value), true
}

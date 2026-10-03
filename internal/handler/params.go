package handler

import (
	"net/http"
	"strconv"
	"time"

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

func parseOccurrenceDate(c *gin.Context) (time.Time, bool) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load timezone"})
		return time.Time{}, false
	}
	date, err := time.ParseInLocation(time.DateOnly, c.Param("date"), location)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid occurrence date"})
		return time.Time{}, false
	}
	return date, true
}

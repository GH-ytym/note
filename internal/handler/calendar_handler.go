package handler

import (
	"errors"
	"net/http"
	apperrors "note/internal/errors"
	"time"

	"github.com/gin-gonic/gin"
)

// GetCalendar returns
func (h *TodoHandler) GetCalendar(c *gin.Context) {
	var query CalendarQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "from and to are required",
		})
		return
	}

	//设置时区
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load timezone",
		})
		return
	}

	//解析from和to
	from, err := time.ParseInLocation(
		time.DateOnly,
		query.From,
		location,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid from date",
		})
		return
	}

	to, err := time.ParseInLocation(
		time.DateOnly,
		query.To,
		location,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid to date",
		})
		return
	}

	//查所有todo和发生时间
	occurrences, err := h.service.CalendarOccurrences(c.Request.Context(), from, to)
	if errors.Is(err, apperrors.ErrInvalidCalendarRange) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load calendar",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": occurrences,
		"from": query.From,
		"to":   query.To,
	})
}

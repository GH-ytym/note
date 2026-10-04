package handler

import (
	"errors"
	"net/http"
	"note/internal/calendar"
	apperrors "note/internal/errors"
	"note/internal/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

type CalendarHandler struct {
	service calendar.Service
}

func NewCalendarHandler(
	service calendar.Service,
) *CalendarHandler {
	return &CalendarHandler{service: service}
}

// GetCalendar returns
func (h *CalendarHandler) GetCalendar(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
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

	//查所有todo和event的出现情况
	result, err := h.service.Get(c.Request.Context(), userID, from, to)
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

	// 数据源已经检查当前群成员资格，这里只保留选中群组的实例。
	if query.GroupID != 0 {
		todos := result.Todos[:0]
		for _, item := range result.Todos {
			if item.GroupID == query.GroupID {
				todos = append(todos, item)
			}
		}
		events := result.Events[:0]
		for _, item := range result.Events {
			if item.GroupID == query.GroupID {
				events = append(events, item)
			}
		}
		result.Todos, result.Events = todos, events
	}
	c.JSON(http.StatusOK, gin.H{
		"data": result,
		"from": query.From,
		"to":   query.To,
	})
}

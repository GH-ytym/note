package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	apperrors "note/internal/errors"
	"note/internal/middleware"
	"note/internal/notification"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	service notification.Service
	hub     *notification.Hub
}

func NewNotificationHandler(
	service notification.Service,
	hub *notification.Hub, //把同一个hub传给handler
) *NotificationHandler {
	return &NotificationHandler{
		service: service,
		hub:     hub,
	}
}

func (h *NotificationHandler) List(c *gin.Context) {
	// 通知包含个人数据，要求浏览器每次重新请求。
	c.Header("Cache-Control", "no-store")

	// 中间件验证 JWT 后保存的当前用户 ID。
	userID := c.GetUint(middleware.UserIDKey)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	result, err := h.service.List(c.Request.Context(), userID)

	switch {
	case errors.Is(err, apperrors.ErrNotificationUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请重新登录",
		})

	case err != nil:
		log.Printf("list notifications for user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询通知失败",
		})

	default:
		c.JSON(http.StatusOK, result)
	}
}

// 持续接收当前窗口的全部events
func (h *NotificationHandler) Stream(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	exp := c.GetTime(middleware.AccessExpiresAtKey)

	if userID == 0 || exp.IsZero() || !time.Now().Before(exp) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "需要重新登录",
		})
		return
	}

	//客户端断开或者jwt到期都需要取消这个ctx
	ctx, cancel := context.WithDeadline(c.Request.Context(), exp)
	defer cancel()

	//给当前连接创建并登记channel
	events, unSub, err := h.hub.Subscribe(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "通知服务暂时不可用",
		})
		return
	}
	defer unSub()

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-store, no-transform")
	c.Header("X-Accel-Buffering", "no")

	controller := http.NewResponseController(c.Writer)

	//把sse写入当前窗口的http相应
	write := func(content string) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		//最多等10s，最晚是exp
		ddl := time.Now().Add(10 * time.Second)
		if exp.Before(ddl) {
			ddl = exp
		}

		if err := controller.SetWriteDeadline(ddl); err != nil {
			return err
		}
		if _, err := io.WriteString(c.Writer, content); err != nil {
			return err
		}

		//发送缓冲内容
		if err := controller.Flush(); err != nil {
			return err
		}

		// 写完后取消写入期限，允许连接继续等待下一条消息。
		return controller.SetWriteDeadline(time.Time{})
	}

	// 立即发送一段 SSE 注释，让客户端确认响应已经开始。
	if err := write(": connected\n\n"); err != nil {
		return
	}

	// 没有业务消息时，也定期发心跳。
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	//一直循环
	for {
		select {
		//不用写default，因为没有消息时，等待是正常的
		case <-ctx.Done():
			// 客户端断开，或者 JWT 到期。
			return

		case event, ok := <-events:
			if !ok {
				//当channel被close的时候，ok就是false
				// 队列已经关闭，而且里面的消息已经读完。
				return
			}

			// 把我们自己的 Event 转成 SSE 格式。
			var content bytes.Buffer
			err := sse.Encode(&content, sse.Event{
				Id:    event.ID,
				Event: event.Name,
				Data:  event.Data,
			})
			if err != nil {
				return
			}

			if err := write(content.String()); err != nil {
				return
			}

		case <-heartbeat.C:
			// SSE 注释，不是一条业务通知。
			if err := write(": ping\n\n"); err != nil {
				return
			}
		}
	}
}

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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

// MarkRead 接收固定的通知 ID 集合，成功时返回 204。
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	uid := c.GetUint(middleware.UserIDKey)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	var req struct {
		IDs []uint `json:"ids" binding:"required,min=1,dive,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids 必须是非空的正整数数组"})
		return
	}
	if err := h.service.MarkRead(c.Request.Context(), uid, req.IDs); err != nil {
		log.Printf("mark notifications read for user %d: %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记已读失败，请重试"})
		return
	}
	c.Status(http.StatusNoContent)
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

	//不传before就是0
	//0就是从最新开始
	before, err := strconv.ParseUint(c.DefaultQuery("before", "0"), 10, strconv.IntSize)
	if err != nil || before > uint64(1<<53-1) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid parameter:before",
		})
		return
	}

	// 不传 limit 时，每页返回 20 条
	limit, err := strconv.ParseUint(
		c.DefaultQuery("limit", "20"),
		10,
		strconv.IntSize,
	)
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "limit 必须是 1 到 100 的整数",
		})
		return
	}

	result, err := h.service.List(
		c.Request.Context(),
		userID,
		uint(before),
		uint(limit),
	)

	switch {
	case errors.Is(err, apperrors.ErrNotificationUnauthenticated):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请重新登录",
		})

	case errors.Is(err, apperrors.ErrCacheUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "通知暂时不可用，请稍后重试",
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
// Stream 先发送数据库第一页快照，再发送变化，并定期检查是否落后。
func (h *NotificationHandler) Stream(c *gin.Context) {
	userID := c.GetUint(middleware.UserIDKey)
	exp := c.GetTime(middleware.AccessExpiresAtKey)

	if userID == 0 || exp.IsZero() || !time.Now().Before(exp) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "需要重新登录",
		})
		return
	}

	ctx, cancel := context.WithDeadline(c.Request.Context(), exp)
	defer cancel()

	// 先登记连接，查询期间的新事件会进入队列。
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

	//实际发送sse文本
	write := func(content string) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		//有效期要么10秒要么access key的ddl
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
		if err := controller.Flush(); err != nil {
			return err
		}

		return controller.SetWriteDeadline(time.Time{})
	}

	//把单次事件转化成sse文本并发送
	send := func(name, data string) error {
		var buf bytes.Buffer
		if err := sse.Encode(&buf, sse.Event{
			Event: name,
			Data:  data,
		}); err != nil {
			return err
		}
		return write(buf.String())
	}

	// 本连接已经发送到的版本。
	var rev uint64

	snapshot := func(min uint64) error {
		readCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()

		page, err := h.service.SnapshotPage(readCtx, userID, 0, 20)
		if err != nil {
			return err
		}

		if page.Revision < min {
			return fmt.Errorf(
				"snapshot revision %d is behind required revision %d",
				page.Revision,
				min,
			)
		}

		data, err := json.Marshal(page)
		if err != nil {
			return err
		}

		if err := send("notification.snapshot", string(data)); err != nil {
			return err
		}

		rev = page.Revision
		return nil
	}

	//第一次调用先查第一页然后发出去
	if err := snapshot(0); err != nil {
		log.Printf(
			"send notification snapshot for user %d: %v",
			userID, err,
		)
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// 定期查询数据库版本，发现没有通过广播收到的变化。
	check := time.NewTicker(60 * time.Second)
	defer check.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-events:
			if !ok {
				return
			}

			switch event.Name {
			//说明发生了缓存重建
			case "notification.reset":
				if err := snapshot(rev); err != nil {
					log.Printf(
						"reset notifications for user %d: %v",
						userID, err,
					)
					return
				}

				//正常情况
			case "notification.change":
				var change notification.Change
				if err := json.Unmarshal(
					[]byte(event.Data), &change,
				); err != nil {
					log.Printf("decode notification change: %v", err)
					return
				}

				//1.当前事件的版本已经覆盖
				//example:
				//业务事务提交，小李申请入群
				//数据库更新到版本12，写入 Outbox
				//    ↓
				//小王连接 SSE
				//    ↓
				//Stream 查数据库，发送版本12的快照
				//rev = 12，页面已经包含“小李申请入群”
				//    ↓
				//Worker 这时才处理到那条 Outbox
				//广播版本12
				//    ↓
				//Stream 收到版本12
				//所以<=rev的说明已经发给user，就不用再发了
				if change.Revision <= rev {
					continue
				}

				//2.版本跳号了
				//example:
				//小王当前 rev = 12
				//    ↓
				//Redis 订阅连接断开
				//    ↓
				//版本13：小张申请入群，广播没收到
				//版本14：另一窗口把某条通知标为已读，广播没收到
				//    ↓
				//Redis 订阅恢复
				//    ↓
				//版本15：小赵申请入群，广播正常收到
				//    ↓
				//Stream 收到版本15
				//这个时候会跳号，跳号就要重新查数据库，发送最新第一页和全部未读数
				if change.Revision != rev+1 {
					if err := snapshot(change.Revision); err != nil {
						log.Printf(
							"resync notifications for user %d: %v",
							userID, err,
						)
						return
					}
					continue
				}

				//3.正常变化，就发送
				if err := send(event.Name, event.Data); err != nil {
					return
				}
				rev = change.Revision
			}

		case <-check.C:
			// 每次检查单独设置查询超时，结束后立即释放。
			readCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			//查最新版本
			current, err := h.service.Revision(readCtx, userID)
			stop()

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf(
					"check notification revision for user %d: %v",
					userID, err,
				)
				// 这次检查失败，下次定时检查继续尝试。
				continue
			}

			if current > rev {
				//发现不对，更新一次
				if err := snapshot(current); err != nil {
					log.Printf(
						"refresh notifications for user %d: %v",
						userID, err,
					)
					return
				}
			}

		case <-heartbeat.C:
			if err := write(": ping\n\n"); err != nil {
				return
			}
		}
	}
}

// Snapshot 返回当前登录用户的数据库完整快照，用于重新同步通知。
func (h *NotificationHandler) Snapshot(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	uid := c.GetUint(middleware.UserIDKey)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "请先登录",
		})
		return
	}

	inbox, err := h.service.Snapshot(c.Request.Context(), uid)
	if err != nil {
		log.Printf("load notification snapshot for user %d: %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询通知失败",
		})
		return
	}

	c.JSON(http.StatusOK, inbox)
}

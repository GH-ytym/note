package router

import (
	"net/http"
	"note/internal/auth"
	"note/internal/middleware"
	"path/filepath"
	"strings"

	"note/internal/handler"
	"note/internal/search"

	"github.com/gin-gonic/gin"
)

// NewWithWeb creates the API router and optionally serves a built React app.
// webDir is empty during normal API development and points to web/dist in Electron.
func NewWithWeb(
	th *handler.TodoHandler,
	eh *handler.EventHandler,
	ch *handler.CalendarHandler,
	sh *search.SearchHandler,
	ah *handler.AuthHandler,
	gh *handler.GroupHandler,
	nh *handler.NotificationHandler,
	tm *auth.TokenManager,
	webDir string,
	profiles ...*handler.ProfileHandler,
) *gin.Engine {
	r := gin.Default()
	registerAPI(r, th, eh, ch, sh, ah, gh, nh, tm)
	registerAPI(r.Group("/api"), th, eh, ch, sh, ah, gh, nh, tm)
	if len(profiles) > 0 && profiles[0] != nil {
		for _, prefix := range []string{"", "/api"} {
			users := r.Group(prefix+"/users", middleware.RequireLogin(tm))
			users.GET("/me", profiles[0].Me)
			users.PUT("/me/avatar", profiles[0].UploadAvatar)
			users.DELETE("/me/avatar", profiles[0].RemoveAvatar)
		}
	}

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

func registerAPI(
	r gin.IRouter,
	th *handler.TodoHandler,
	eh *handler.EventHandler,
	ch *handler.CalendarHandler,
	sh *search.SearchHandler,
	ah *handler.AuthHandler,
	gh *handler.GroupHandler,
	nh *handler.NotificationHandler,
	tm *auth.TokenManager,
) {
	// 公开接口：不需要登录。
	r.POST("/auth/login", middleware.RequireAuthRequest(), ah.Login)
	r.POST("/auth/register", ah.Register)
	// 刷新使用 Cookie 验证身份，不能经过 access JWT 中间件。
	r.POST("/auth/refresh", middleware.RequireAuthRequest(), ah.Refresh)
	r.POST("/auth/logout", middleware.RequireAuthRequest(), ah.Logout)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// 受保护接口：先经过 RequireLogin。
	protected := r.Group("")
	// RequireLogin 读取前端携带的 access token，验证后保存当前用户 ID。
	protected.Use(middleware.RequireLogin(tm))

	// 通知属于接收者；尚未入群的用户也需要查看发给自己的邀请。
	protected.GET("/notifications", nh.List)
	//循环写sse和接收events
	protected.GET("/notifications/stream", nh.Stream)
	protected.GET("/calendar", ch.GetCalendar)

	groups := protected.Group("/groups")
	{
		groups.POST("", gh.CreateGroup)
		groups.GET("", gh.MyGroups)
		groups.GET("/:groupID/members", gh.ListMembers)
		groups.GET("/:groupID/todos", th.ListTodos)
		groups.GET("/:groupID/events", eh.ListEvents)
		// 群组搜索沿用 RequireLogin，并在仓库内校验当前成员资格。
		groups.GET("/:groupID/search/todos", sh.SearchTodos)
		groups.GET("/:groupID/search/events", sh.SearchEvents)
		groups.GET("/:groupID/search/all", sh.SearchAll)
		// 群号公开且固定，查询及加入仍要求登录。

		//加群和退群
		groups.GET("/lookup", gh.Lookup)
		groups.POST("/join", gh.JoinGroup)
		groups.POST("/:groupID/quit", gh.QuitGroup)
		//解散群组
		groups.POST("/:groupID/dismiss", gh.DismissGroup)
	}

	todos := protected.Group("/todos")
	{
		todos.POST("", th.CreateTodo)

		todos.GET("/:id", th.GetTodo)
		todos.PATCH("/:id", th.PatchTodo)
		//将某一条todo的某一天设置为完成或未完成
		todos.PATCH(
			"/:id/occurrences/:date",
			th.PatchOccurrenceDone,
		)
		//查看某一条todo某一天有谁完成了
		todos.GET(
			"/:id/occurrences/:date/completions",
			th.GetOccurrenceCompletions,
		)
		todos.DELETE("/:id", th.DeleteTodo)

		todos.PATCH("/:id/members", th.PatchRoles)
	}

	events := protected.Group("/events")
	{
		events.POST("", eh.CreateEvent)
		events.GET("/:id", eh.GetEvent)
		events.PATCH("/:id", eh.PatchEvent)
		events.DELETE("/:id", eh.DeleteEvent)
		events.PATCH("/:id/members", eh.PatchRoles)
	}
}

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
	tm *auth.TokenManager,
	webDir string,
) *gin.Engine {
	r := gin.Default()
	registerAPI(r, th, eh, ch, sh, ah, gh, tm)
	registerAPI(r.Group("/api"), th, eh, ch, sh, ah, gh, tm)

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
	//这一步后，前端请求会携带：Authorization: Bearer <登录得到的Token>
	protected.Use(middleware.RequireLogin(tm))

	protected.GET("/calendar", ch.GetCalendar)
	protected.GET("/search/todos", sh.SearchTodos)
	protected.GET("/search/events", sh.SearchEvents)
	protected.GET("/search/all", sh.SearchAll)

	groups := protected.Group("/groups")
	{
		groups.POST("", gh.CreateGroup)
		groups.GET("", gh.MyGroups)
		groups.GET("/:groupID/todos", th.ListTodos)
		//群成员获取邀请码，只有群主可以刷新。
		groups.GET("/:groupID/invite", gh.GetInviteCode)
		groups.POST("/:groupID/refresh", gh.RefreshInviteCode)

		//加群和退群
		groups.POST("/:groupID/join", gh.JoinGroup)
		groups.POST("/:groupID/quit", gh.QuitGroup)
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
	}

	events := protected.Group("/events")
	{
		events.POST("", eh.CreateEvent)
		events.GET("/:id", eh.GetEvent)
		events.PATCH("/:id", eh.PatchEvent)
	}
}

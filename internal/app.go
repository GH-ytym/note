package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"note/internal/handler"
	"note/internal/model"
	"note/internal/retry"
	"note/internal/router"
	"note/internal/todo"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://note:note@localhost:5433/note?sslmode=disable"
	}

	db, err := gorm.Open(
		postgres.Open(databaseURL),
		&gorm.Config{
			TranslateError: true,
		})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql database: %w", err)
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = retry.Do(ctx, func() error {
		return sqlDB.PingContext(ctx)
	})
	cancel()
	if err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	//同步表结构
	if err := db.AutoMigrate(
		&model.Todo{},
		&model.TodoDate{},
		&model.TodoCompletion{},
	); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// 静态类型是 todo.Repository；实际值是隐藏的 *todo.gormRepository。
	todoRepository := todo.NewGORMRepository(db)

	// 静态类型是 todo.TodoService；实际值是隐藏的 *todo.service。
	todoService := todo.NewService(todoRepository)

	// 静态类型是 *handler.Handler。
	// Handler 只保存 todo.TodoService，不接触具体的 service 实现。
	hdlr := handler.NewHandler(todoService)

	//启动服务+优雅关闭
	server := &http.Server{
		Addr:              ":8080",
		Handler:           router.New(hdlr),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 使用容量为 1 的缓冲 channel。
	// Shutdown 会使 ListenAndServe 返回 http.ErrServerClosed。
	// 此时主 goroutine 可能已经离开 select；如果使用无缓冲 channel，
	// 服务 goroutine 可能因为没有接收者而永久阻塞。
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on %s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	//监听关闭信号
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-stopCtx.Done():
		log.Print("shutdown signal received")
	}

	//最多等待10秒
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	// 尝试优雅关闭 HTTP 服务
	if err := server.Shutdown(shutdownCtx); err != nil {
		// 超时后强制关闭 HTTP 连接
		_ = server.Close()
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	log.Print("HTTP server stopped")
	return nil
}

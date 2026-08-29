package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"note/internal/handler"
	"note/internal/model"
	"note/internal/retry"
	"note/internal/router"
	"note/internal/todo"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func Run() (runErr error) {
	databasePath := os.Getenv("NOTE_DB_PATH")
	if databasePath == "" {
		databasePath = filepath.Join("data", "note.db")
	}

	databasePath, err := prepareDatabasePath(databasePath)
	if err != nil {
		return err
	}

	db, err := gorm.Open(
		gormlite.Open(sqliteDSN(databasePath)),
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
	defer func() {
		log.Print("closing SQLite database")
		if err := sqlDB.Close(); runErr == nil && err != nil {
			runErr = fmt.Errorf("close database: %w", err)
		}
	}()

	// SQLite 同一时刻只有一个写入者。桌面应用的数据量很小，
	// 使用一个连接可以让多个窗口的写操作在进程内自然排队。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

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
	log.Printf("SQLite database: %s", databasePath)

	// 静态类型是 todo.Repository；实际值是隐藏的 *todo.gormRepository。
	todoRepository := todo.NewGORMRepository(db)

	// 静态类型是 todo.TodoService；实际值是隐藏的 *todo.service。
	todoService := todo.NewService(todoRepository)

	// 静态类型是 *handler.Handler。
	// Handler 只保存 todo.TodoService，不接触具体的 service 实现。
	hdlr := handler.NewHandler(todoService)

	//启动服务+优雅关闭
	serverAddress := os.Getenv("HTTP_ADDR")
	if serverAddress == "" {
		serverAddress = "127.0.0.1:8080"
	}

	server := &http.Server{
		Addr:              serverAddress,
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

func prepareDatabasePath(databasePath string) (string, error) {
	absolutePath, err := filepath.Abs(databasePath)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return "", fmt.Errorf("create database directory: %w", err)
	}

	return absolutePath, nil
}

func sqliteDSN(databasePath string) string {
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(on)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "synchronous(NORMAL)")
	query.Set("_txlock", "immediate")

	return "file:" + filepath.ToSlash(databasePath) + "?" + query.Encode()
}

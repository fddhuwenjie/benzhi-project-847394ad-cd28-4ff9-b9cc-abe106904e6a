package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func serve(cfg config, logger *slog.Logger) error {
	rt, err := buildRuntime(cfg.DataPath, logger)
	if err != nil {
		return fmt.Errorf("初始化服务: %w", err)
	}
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.Addr, err)
	}
	server := newHTTPServer(cfg.Addr, rt.handler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	logger.Info("受限空间作业许可服务已启动", "addr", listener.Addr().String(), "data", cfg.DataPath)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		}
		return nil
	case <-ctx.Done():
		logger.Info("正在关闭服务")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("优雅关闭 HTTP 服务: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP 服务关闭异常: %w", err)
	}
	if err := rt.service.Flush(shutdownCtx); err != nil {
		return fmt.Errorf("刷新持久化快照: %w", err)
	}
	return nil
}

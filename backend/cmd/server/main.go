package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/worryzyy/upstream-hub/internal/api"
	"github.com/worryzyy/upstream-hub/internal/auth"
	"github.com/worryzyy/upstream-hub/internal/channel"
	"github.com/worryzyy/upstream-hub/internal/config"
	"github.com/worryzyy/upstream-hub/internal/crypto"
	"github.com/worryzyy/upstream-hub/internal/logger"
	"github.com/worryzyy/upstream-hub/internal/monitor"
	"github.com/worryzyy/upstream-hub/internal/notify"
	"github.com/worryzyy/upstream-hub/internal/scheduler"
	"github.com/worryzyy/upstream-hub/internal/storage"

	// 注册 connector 实现。
	_ "github.com/worryzyy/upstream-hub/internal/connector/newapi"
	_ "github.com/worryzyy/upstream-hub/internal/connector/sub2api"
)

func main() {
	configPath := flag.String("config", "", "path to config.yaml (optional; env vars also supported)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	log.Info("starting upstream-hub", "port", cfg.Server.Port, "mode", cfg.Server.Mode)

	cipher, err := crypto.NewCipher(cfg.Security.AppSecret)
	if err != nil {
		log.Error("init cipher failed (set APP_SECRET)", "err", err)
		os.Exit(1)
	}

	// Auth：账号密码取自 config，token secret 缺省回退到 AppSecret。
	tokenSecret := cfg.Auth.TokenSecret
	if tokenSecret == "" {
		tokenSecret = cfg.Security.AppSecret
	}
	authSvc, err := auth.New(
		cfg.Auth.Username,
		cfg.Auth.Password,
		tokenSecret,
		time.Duration(cfg.Auth.SessionTTLHours)*time.Hour,
	)
	if err != nil {
		log.Error("init auth failed (set ADMIN_USERNAME / ADMIN_PASSWORD)", "err", err)
		os.Exit(1)
	}

	db, err := storage.Open(cfg.Database.ToStorageConfig())
	if err != nil {
		log.Error("open database failed", "err", err)
		os.Exit(1)
	}
	if err := storage.AutoMigrate(db); err != nil {
		log.Error("auto migrate failed", "err", err)
		os.Exit(1)
	}

	channels := storage.NewChannels(db)
	authSessions := storage.NewAuthSessions(db)
	captchas := storage.NewCaptchas(db)
	notifies := storage.NewNotifications(db)
	rates := storage.NewRates(db)
	monLogs := storage.NewMonitorLogs(db)

	channelSvc := channel.NewService(channels, authSessions, captchas, monLogs, cipher)
	dispatcher := notify.NewDispatcher(notifies, cipher, log)
	monitorSvc := monitor.NewService(channels, rates, monLogs, channelSvc, dispatcher, log)

	sch := scheduler.New(cfg.Scheduler, monitorSvc, monLogs, rates, notifies, log)
	if err := sch.Start(); err != nil {
		log.Error("start scheduler failed", "err", err)
		os.Exit(1)
	}
	defer sch.Stop()

	gin.SetMode(cfg.Server.Mode)
	router := gin.New()
	router.Use(gin.Recovery())
	if len(cfg.Server.TrustedProxies) > 0 {
		_ = router.SetTrustedProxies(cfg.Server.TrustedProxies)
	}

	api.Register(router, &api.Deps{
		DB:         db,
		Cipher:     cipher,
		Auth:       authSvc,
		Channels:   channels,
		Sessions:   authSessions,
		Captchas:   captchas,
		Notifies:   notifies,
		Rates:      rates,
		MonLogs:    monLogs,
		ChannelSvc: channelSvc,
		Monitor:    monitorSvc,
		Dispatcher: dispatcher,
		Log:        log,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()
	log.Info("http server listening", "addr", srv.Addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("http shutdown error", "err", err)
	}
	log.Info("bye")
}

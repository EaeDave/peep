package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"

	apihttp "peep/api/internal/http"
	"peep/api/internal/rooms"
)

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("missing env", "var", k)
		os.Exit(1)
	}
	return v
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := apihttp.Config{
		APIKey:       mustEnv("LIVEKIT_API_KEY"),
		APISecret:    mustEnv("LIVEKIT_API_SECRET"),
		InviteSecret: mustEnv("INVITE_SECRET"),
		PublicWSURL:  mustEnv("PUBLIC_LIVEKIT_WS_URL"),
		// Tauri origins are always allowed (Windows client).
		AllowedOrigins: []string{"tauri://localhost", "http://tauri.localhost"},
		AdminUser:      os.Getenv("ADMIN_USER"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
	}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	lk := lksdk.NewRoomServiceClient(mustEnv("LIVEKIT_URL"), cfg.APIKey, cfg.APISecret)
	srv := apihttp.New(cfg, rooms.NewStore(), lk)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go srv.SweepLoop(ctx, 60*time.Second)

	hs := &http.Server{Addr: ":" + port, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	slog.Info("listening", "port", port, "origins", cfg.AllowedOrigins, "admin", cfg.AdminUser != "" && cfg.AdminPassword != "")
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

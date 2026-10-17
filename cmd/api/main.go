package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Davi-OS/go-auth-service/pkg/authsdk"

	"github.com/Davi-OS/rain-saver/internal/config"
	rainhttp "github.com/Davi-OS/rain-saver/internal/http"
	"github.com/Davi-OS/rain-saver/internal/repository/postgres"
	"github.com/Davi-OS/rain-saver/internal/service"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.DSN())
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	authClient, err := authsdk.New(ctx, authsdk.Config{
		BaseURL:  cfg.AuthServiceBaseURL,
		Audience: "rain-saver",
	})
	if err != nil {
		log.Fatalf("connect to auth service: %v", err)
	}
	defer authClient.Close()

	enderecoRepo := postgres.NewEnderecoRepository(pool)
	economyRepo := postgres.NewPluvialEconomyRepository(pool)
	stationRepo := postgres.NewWeatherStationRepository(pool)

	enderecoSvc := service.NewEnderecoService(enderecoRepo)
	pluvialSvc := service.NewPluvialService(enderecoRepo, economyRepo, stationRepo)

	handler := rainhttp.NewServer(enderecoSvc, pluvialSvc, authClient, cfg.CORSAllowedOrigins)

	httpServer := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("rain-saver listening on %s (docs at %s/docs)", cfg.Addr(), cfg.Addr())
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
}

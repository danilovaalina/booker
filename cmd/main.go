package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"booker/internal/api"
	"booker/internal/config"
	"booker/internal/db/postgres"
	"booker/internal/repository"
	"booker/internal/sender"
	"booker/internal/service"

	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Stack().Err(err).Send()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Pool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Stack().Err(err).Send()
	}
	defer pool.Close()

	r := repository.New(pool)
	s, err := sender.New(
		//sender.WithTelegram(cfg.BotToken),
		sender.WithEmail(cfg.Email.Host, cfg.Email.Port, cfg.Email.User, cfg.Email.Password, cfg.Email.From),
	)
	if err != nil {
		log.Fatal().Stack().Err(err).Send()
	}

	svc := service.New(service.Options{
		Repo:           r,
		Sender:         s,
		DefaultTimeout: cfg.DefaultTimeout,
	})
	svc.StartCleanup(ctx, cfg.CleanupInterval)

	a := api.New(svc)
	go func() {
		if err = a.Start(cfg.Addr); err != nil {
			log.Fatal().Stack().Err(err).Send()
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = a.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Send()
	}

}

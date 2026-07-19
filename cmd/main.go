package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"websockethub/cmd/server"

	"github.com/rs/zerolog"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	output := zerolog.ConsoleWriter{Out: os.Stdout}
	logger := zerolog.New(output).
		Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger()

	srv, err := server.NewServer(
		server.WithLogger(logger),
	)
	if err != nil {
		logger.Fatal().AnErr("Failed to setup server", err).Send()
	}
	err = srv.Start()
	if err != nil {
		logger.Fatal().AnErr("Server failed", err).Send()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		logger.Info().Msg("failed to shutdown server")
	}
	logger.Info().Msg("Shutting down server...")
}

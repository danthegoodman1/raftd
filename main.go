package main

import (
	"context"
	"errors"
	"github.com/danthegoodman1/raftd/env"
	"github.com/danthegoodman1/raftd/observability"
	"github.com/danthegoodman1/raftd/raft"
	"github.com/danthegoodman1/raftd/syncx"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danthegoodman1/raftd/gologger"
	"github.com/danthegoodman1/raftd/http_server"
)

var logger = gologger.NewLogger()

func main() {
	logger.Debug().Msg("starting raftd")

	prometheusReporter := observability.NewPrometheusReporter()
	go func() {
		err := observability.StartInternalHTTPServer(env.MetricsAPIListenAddr, prometheusReporter)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("internal server couldn't start")
			return
		}
	}()

	readyMap := syncx.NewMap[uint64, bool]()

	raftManager, err := raft.NewRaftManager(&readyMap)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create new raft manager")
		return
	}

	httpServer := http_server.StartHTTPServer(&readyMap, raftManager)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	logger.Warn().Msg("received shutdown signal!")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("failed to shutdown HTTP server")
	} else {
		logger.Info().Msg("successfully shutdown HTTP server")
	}

	err = raftManager.Shutdown()
	if err != nil {
		logger.Fatal().Err(err).Msg("error shutting down raft manager")
		return
	}
}

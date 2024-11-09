package http_server

import (
	"context"
	"errors"
	"fmt"
	"github.com/danthegoodman1/raftd/env"
	"github.com/danthegoodman1/raftd/raft"
	"github.com/danthegoodman1/raftd/syncx"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/danthegoodman1/raftd/gologger"
	"github.com/danthegoodman1/raftd/utils"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
)

var logger = gologger.NewLogger().With().Str("Service", "HTTPServer").Logger()

type HTTPServer struct {
	Echo    *echo.Echo
	manager *raft.RaftManager
	Ready   *syncx.Map[uint64, bool]
}

type CustomValidator struct {
	validator *validator.Validate
}

func StartHTTPServer(readyMap *syncx.Map[uint64, bool], manager *raft.RaftManager) *HTTPServer {
	listener, err := net.Listen("tcp", env.HTTPListenAddr)
	if err != nil {
		logger.Error().Err(err).Msg("error creating tcp listener, exiting")
		os.Exit(1)
	}
	s := &HTTPServer{
		Echo:  echo.New(),
		Ready: readyMap,
	}
	s.Echo.HideBanner = true
	s.Echo.HidePort = true
	s.Echo.JSONSerializer = &utils.NoEscapeJSONSerializer{}

	s.manager = manager

	s.Echo.Use(CreateReqContext)
	s.Echo.Use(LoggerMiddleware)
	s.Echo.Use(middleware.CORS())
	s.Echo.Validator = &CustomValidator{validator: validator.New()}
	s.Echo.HTTPErrorHandler = customHTTPErrorHandler

	s.Echo.GET("/hc", s.HealthCheck)
	s.Echo.GET("/rc", s.ReadinessCheck)

	{
		// Data operations
		raftGroup := s.Echo.Group("/raft")
		raftGroup.GET("/read", ccHandler(s.Lookup))
		raftGroup.POST("/update", ccHandler(s.Update))
		raftGroup.GET("/snapshot", ccHandler(s.ReadSnapshot))
		raftGroup.POST("/snapshot", ccHandler(s.CreateSnapshot))

		// Raft management
		raftGroup.POST("/recruit_replica", ccHandler(s.RecruitReplica))
		raftGroup.POST("/remove_replica", ccHandler(s.RemoveReplica))
		// todo POST /new_shard
		// todo GET /membership_info
	}

	s.Echo.Listener = listener
	go func() {
		logger.Info().Msg("starting h2c server on " + listener.Addr().String())
		// this just basically creates an h2c.NewHandler(echo, &http2.Server{})
		err := s.Echo.StartH2CServer("", &http2.Server{})
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("failed to start h2c server, exiting")
			os.Exit(1)
		}
	}()

	return s
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func ValidateRequest(c echo.Context, s interface{}) error {
	if err := c.Bind(s); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	// needed because POST doesn't have query param binding (https://echo.labstack.com/docs/binding#multiple-sources)
	if err := (&echo.DefaultBinder{}).BindQueryParams(c, s); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(s); err != nil {
		return err
	}
	return nil
}

func (s *HTTPServer) HealthCheck(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func (s *HTTPServer) ReadinessCheck(c echo.Context) error {
	// TODO check all shards for readiness
	readyCode := s.Ready.Load()
	switch readyCode {
	case 0:
		return c.String(http.StatusInternalServerError, "not ready")
	case 1:
		return c.String(http.StatusOK, "ready")
	case 2:
		return c.String(http.StatusInternalServerError, "shut down")
	default:
		return c.String(http.StatusInternalServerError, fmt.Sprintf("unknown status code %d", readyCode))
	}

}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	err := s.Echo.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error shutting down echo: %w", err)
	}

	return nil
}

func LoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		if err := next(c); err != nil {
			// default handler
			c.Error(err)
		}
		stop := time.Since(start)
		// Log otherwise
		logger := zerolog.Ctx(c.Request().Context())
		req := c.Request()
		res := c.Response()

		p := req.URL.Path
		if p == "" {
			p = "/"
		}

		cl := req.Header.Get(echo.HeaderContentLength)
		if cl == "" {
			cl = "0"
		}
		logger.Debug().Str("method", req.Method).Str("remote_ip", c.RealIP()).Str("req_uri", req.RequestURI).Str("handler_path", c.Path()).Str("path", p).Int("status", res.Status).Int64("latency_ns", int64(stop)).Str("protocol", req.Proto).Str("bytes_in", cl).Int64("bytes_out", res.Size).Msg("req recived")
		return nil
	}
}

package http_server

import (
	"context"
	"errors"
	"fmt"
	"github.com/danthegoodman1/raftd/raft"
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
}

type CustomValidator struct {
	validator *validator.Validate
}

func StartHTTPServer(manager *raft.RaftManager) *HTTPServer {
	listener, err := net.Listen("tcp", utils.HTTPListenAddr)
	if err != nil {
		logger.Error().Err(err).Msg("error creating tcp listener, exiting")
		os.Exit(1)
	}
	s := &HTTPServer{
		Echo: echo.New(),
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

	// todo write record
	// todo read record
	// todo request snapshot
	// todo read snapshot?
	// todo group management

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

func (*HTTPServer) HealthCheck(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func (*HTTPServer) ReadinessCheck(c echo.Context) error {
	return c.String(http.StatusOK, "ok")
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

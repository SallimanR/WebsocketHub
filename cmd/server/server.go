package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SallimanR/websockethub/websockethub"
	wsPB "github.com/SallimanR/websockethub/websockethub/proto"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const defaultHTTPPort = 8080

type Server struct {
	httpAddrs  string
	httpRouter *gin.Engine
	httpAPI    *gin.RouterGroup

	httpServer *http.Server

	wsServer       *websockethub.WebsocketServer
	authMiddleware gin.HandlerFunc
	logger         zerolog.Logger
	config         *Config
}

type Option func(*Server) error

func NewServer(options ...Option) (*Server, error) {
	s := &Server{
		httpAddrs: fmt.Sprintf("127.0.0.1:%d", defaultHTTPPort),
	}

	if s.httpRouter == nil {
		s.httpRouter = gin.New()
		s.httpRouter.Use(gin.Logger())
		s.httpRouter.Use(cors.New(newCORSConfig()))
	}

	if s.httpAPI == nil {
		s.httpAPI = s.httpRouter.Group("/api/")
	}

	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}

	if s.httpServer == nil {
		s.httpServer = &http.Server{
			Addr:    s.httpAddrs,
			Handler: s.httpRouter,
		}
	}

	return s, nil
}

func WithConfig(cfg *Config) Option {
	return func(s *Server) error {
		s.config = cfg
		s.httpAddrs = fmt.Sprintf("127.0.0.1:%d", cfg.Port)
		return nil
	}
}

func WithAuthMiddleware(mw gin.HandlerFunc) Option {
	return func(s *Server) error {
		s.authMiddleware = mw
		return nil
	}
}

func WithLogger(logger zerolog.Logger) Option {
	return func(s *Server) error {
		s.logger = logger
		return nil
	}
}

func (s *Server) Start() error {
	s.setupMonitoringRoutes()
	go s.startListening()

	s.registerWSRoutes()
	s.startWSServer()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("Shutting down HTTP server...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error().AnErr("HTTP shutdown error: %v", err).Send()
		return err
	}
	s.logger.Info().Msg("HTTP server shut down")
	return nil
}

func (s *Server) setupMonitoringRoutes() {
	s.httpRouter.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"status": "ok"})
	})
}

func (s *Server) startListening() {
	s.logger.Info().Str("Server starting on", s.httpAddrs).Send()
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		s.logger.Fatal().AnErr("Server failed:", err).Send()
	}
}

func (s *Server) registerWSRoutes() {
	if s.wsServer == nil {
		roles := []string{"tow_driver", "tow_subscriber"}
		if s.config != nil && len(s.config.Roles) > 0 {
			roles = s.config.Roles
		}

		allowedOrigins := loadAllowedOrigins()
		wsOptions := websockethub.WebsocketServerOptions{
			Roles:          roles,
			AllowedOrigins: allowedOrigins,
			Logger:         s.logger,
		}
		s.wsServer = websockethub.NewWebsocketServer(wsOptions)
		s.registerChannels()
	}
	wsGroup := s.httpRouter.Group("/ws")
	if s.authMiddleware != nil {
		wsGroup.Use(s.authMiddleware)
	}
	wsGroup.GET("/:role", s.wsServer.WebsocketUpgradeHandler)
}

func (s *Server) registerChannels() {
	if s.config == nil {
		return
	}
	for _, chCfg := range s.config.Channels {
		v, ok := wsPB.Channel_value[chCfg.Name]
		if !ok {
			s.logger.Warn().Str("channel", chCfg.Name).Msg("unknown channel name, skipping")
			continue
		}
		ch := wsPB.Channel(v)
		pubSub := newRawChannel()
		if err := s.wsServer.RegisterChannel(chCfg.Roles, ch, pubSub); err != nil {
			s.logger.Error().Str("channel", chCfg.Name).Err(err).Msg("failed to register channel")
		} else {
			s.logger.Info().Str("channel", chCfg.Name).Msg("registered channel")
		}
	}
}

func (s *Server) startWSServer() {
	s.wsServer.Run()
}

func loadCORSOrigins() []string {
	raw := os.Getenv("ALLOWED_CORS_ORIGINS")
	if raw == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

func newCORSConfig() cors.Config {
	origins := loadCORSOrigins()
	if len(origins) == 0 {
		return cors.Config{
			AllowOriginFunc:  func(origin string) bool { return true },
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "Cookie"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}
	}
	return cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "Cookie"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

func loadAllowedOrigins() []string {
	raw := os.Getenv("ALLOWED_WS_ORIGINS")
	if raw == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

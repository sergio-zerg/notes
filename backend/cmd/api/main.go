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
	"sync"
	"syscall"
	"time"

	"notes/api/proto"
	"notes/internal/application"
	"notes/internal/config"
	"notes/internal/infrastructure/observability"
	"notes/internal/infrastructure/persistence"
	"notes/internal/infrastructure/persistence/postgres"
	grpctransport "notes/internal/infrastructure/transport/grpc"
	grpcmiddleware "notes/internal/infrastructure/transport/grpc/middleware"
	httptransport "notes/internal/infrastructure/transport/http"
	"notes/internal/infrastructure/transport/http/middleware"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

const (
	// Time budget for in-flight requests to finish on shutdown.
	shutdownTimeout = 10 * time.Second
	// Header guarding against slowloris-style clients.
	readHeaderTimeout = 5 * time.Second
	// Idle keep-alive window.
	readTimeout  = 15 * time.Second
	writeTimeout = 30 * time.Second
	// How often idle rate-limit buckets are swept, and how long a bucket
	// survives without traffic.
	cleanupInterval = 5 * time.Minute
	bucketTTL       = 10 * time.Minute
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 2. Observability
	shutdownTracer, err := observability.InitTracer(observability.TracerConfig{
		ServiceName:  "notes-service",
		Exporter:     observability.TraceExporter(cfg.TraceExporter),
		OTLPEndpoint: cfg.OTLPEndpoint,
		SampleRatio:  cfg.TraceSampleRate,
	})
	if err != nil {
		// Tracing is best-effort: a failure here must not stop the service.
		logger.Error("failed to init tracer", "error", err)
	} else {
		defer func() {
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdownTracer(flushCtx); err != nil {
				logger.Error("failed to flush traces", "error", err)
			}
		}()
	}

	// 3. Database (also runs migrations)
	db, err := persistence.NewPostgresDB(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() { _ = db.Close() }()

	// 4. Layers
	dao := postgres.NewNoteDAO(db)
	repo := postgres.NewPostgresNoteRepository(dao)
	service := application.NewNoteService(repo)

	// 5. HTTP transport
	httpHandler := httptransport.NewNoteHandler(service)
	healthHandler := httptransport.NewHealthHandler(db)
	httpRouter := mux.NewRouter()
	// Runs after route matching, so the outer middleware can label metrics and
	// spans with "/notes/{id}" instead of every concrete id.
	httpRouter.Use(middleware.CaptureRouteMiddleware)
	httptransport.RegisterRoutes(httpRouter, httpHandler, healthHandler)

	// /metrics is served outside the instrumented chain. Registering it on the
	// same router would still place it behind MetricsMiddleware, so every
	// Prometheus scrape would be counted as application traffic. It also skips
	// authentication, since a scrape must not need a credential.
	promHandler := promhttp.Handler()

	// Liveness checks stay open so orchestrators and Docker can probe the
	// service without holding a key.
	auth := middleware.NewAPIKeyAuthenticator(
		cfg.APIKeys,
		cfg.AllowInsecure,
		"/healthz",
	)

	limiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	stopCleanup := limiter.StartCleanup(ctx, cleanupInterval, bucketTTL)
	defer stopCleanup()

	instrumented := http.Handler(httpRouter)
	// Applied bottom-up, so the execution order is the reverse: validation
	// runs first, then recovery, then logging, metrics and finally tracing.
	instrumented = middleware.TracingMiddleware(instrumented)
	instrumented = middleware.MetricsMiddleware(instrumented)
	instrumented = middleware.LoggingMiddleware(logger)(instrumented)
	instrumented = middleware.RecoveryMiddleware(logger)(instrumented)
	instrumented = middleware.ValidationMiddleware(instrumented)
	// Auth and rate limiting sit outermost: reject unwanted work before it
	// costs a span or a metrics series.
	instrumented = auth.AuthMiddleware(instrumented)
	instrumented = limiter.RateLimitMiddleware(instrumented)

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			promHandler.ServeHTTP(w, r)
			return
		}
		instrumented.ServeHTTP(w, r)
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPPort,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
	}

	// 6. gRPC transport. The same rules apply as over HTTP: the caller must
	// present a key, and bursts beyond the budget are rejected.
	authorizer := grpcmiddleware.NewAuthorizer(cfg.APIKeys, cfg.AllowInsecure)
	grpcLimiter := grpcmiddleware.NewGrpcRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	grpcLimiter.StartCleanup(ctx, cleanupInterval, bucketTTL)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcmiddleware.GrpcMetricsInterceptor(),
			authorizer.AuthorizeUnaryInterceptor(),
			grpcLimiter.RateLimitUnaryInterceptor(),
		),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	proto.RegisterNoteServiceServer(grpcServer, grpctransport.NewNoteHandler(service))

	// 7. Serve both transports, and report the first fatal error.
	errCh := make(chan error, 2)

	// The gRPC listener is opened synchronously so that a port conflict is
	// reported before the process claims to be running.
	grpcLis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("gRPC listen on %s: %w", cfg.GRPCPort, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logger.Info("HTTP server listening", "addr", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		logger.Info("gRPC server listening", "addr", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("grpc server: %w", err)
		}
	}()

	// Wait for a signal or a server failure, whichever comes first.
	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case serveErr = <-errCh:
		logger.Error("server failed, shutting down", "error", serveErr)
	}

	// Graceful shutdown, bounded by shutdownTimeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown error", "error", err)
	}

	// GracefulStop blocks until in-flight RPCs finish, so it needs a bound
	// of its own: run it and fall back to Stop if it overruns.
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		logger.Warn("gRPC graceful stop timed out, forcing")
		grpcServer.Stop()
	}

	wg.Wait()
	logger.Info("servers stopped")

	return serveErr
}

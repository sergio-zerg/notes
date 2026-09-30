package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notes/api/proto"
	"notes/internal/application"
	"notes/internal/config"
	"notes/internal/infrastructure/observability"
	"notes/internal/infrastructure/persistence"
	"notes/internal/infrastructure/persistence/sqlite"
	httptransport "notes/internal/infrastructure/transport/http"
	"notes/internal/infrastructure/transport/http/middleware"
	grpctransport "notes/internal/infrastructure/transport/grpc"
	grpcmiddleware "notes/internal/infrastructure/transport/grpc/middleware"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

func main() {
	// 1. Configuration
	cfg := config.Load()

	// 2. Logging
	logger := slog.Default()

	// 3. Observability Initialization
	shutdownTracer, err := observability.InitTracer("notes-service")
	if err != nil {
		logger.Error("failed to init tracer", "error", err)
	}
	defer func() {
		if shutdownTracer != nil {
			shutdownTracer(context.Background())
		}
	}()

	// 4. Infrastructure: SQLite DB
	db, err := persistence.NewSQLiteDB(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 5. Persistence Layer
	dao := sqlite.NewNoteDAO(db)
	repo := sqlite.NewSQLiteNoteRepository(dao)

	// 6. Application Layer: Service
	service := application.NewNoteService(repo)

	// 7. Transport: HTTP
	httpHandler := httptransport.NewNoteHandler(service)
	httpRouter := mux.NewRouter()
	httptransport.RegisterRoutes(httpRouter, httpHandler)

	// Apply Observability & Security Middleware
	var handler http.Handler = httpRouter
	handler = middleware.TracingMiddleware(handler)
	handler = middleware.MetricsMiddleware(handler)
	handler = middleware.LoggingMiddleware(logger)(handler)
	handler = middleware.RecoveryMiddleware(logger)(handler)
	handler = middleware.ValidationMiddleware(handler)

	// Metrics endpoint for Prometheus
	httpRouter.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    cfg.HTTPPort,
		Handler: handler,
	}

	// 8. Transport: gRPC
	grpcHandler := grpctransport.NewNoteHandler(service)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcmiddleware.GrpcMetricsInterceptor()),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	proto.RegisterNoteServiceServer(grpcServer, grpcHandler)

	// 9. Run Servers concurrently
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("HTTP server starting on %s...\n", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
		}
	}()

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCPort)
		if err != nil {
			logger.Error("gRPC listen failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("gRPC server starting on %s...\n", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server failed", "error", err)
		}
	}()

	<-stop
	fmt.Println("\nShutting down servers...")

	// Graceful shutdown for HTTP
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP shutdown error", "error", err)
	}

	// Graceful shutdown for gRPC
	grpcServer.GracefulStop()

	fmt.Println("Server stopped.")
}

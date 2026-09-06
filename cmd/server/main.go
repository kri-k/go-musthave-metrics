package main

import (
	"context"
	"database/sql"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/kri-k/go-musthave-metrics/internal/config/db"
	"github.com/kri-k/go-musthave-metrics/internal/handler"
	"github.com/kri-k/go-musthave-metrics/internal/logger"
	"github.com/kri-k/go-musthave-metrics/internal/middleware"
	"github.com/kri-k/go-musthave-metrics/internal/repository"
	"github.com/kri-k/go-musthave-metrics/internal/service"
	"github.com/kri-k/go-musthave-metrics/internal/util"
)

var (
	flagAddr          = flag.String("a", "localhost:8080", "address and port to run server")
	flagStoreInterval = flag.Int("i", 300, "interval in seconds for saving metrics to disk (0 = sync)")
	flagFileStorage   = flag.String("f", "./metrics-db.json", "path to file for metrics storage")
	flagRestore       = flag.Bool("r", true, "restore previously saved metrics on startup")
	flagDatabaseDSN   = flag.String("d", "", "PostgreSQL connection string (DATABASE_DSN)")
)

func main() {
	logger.Initialize("INFO")
	defer logger.Log.Sync()

	flag.Parse()

	addr := util.GetEnvOrDefaultString("ADDRESS", *flagAddr)
	fileStoragePath := util.GetEnvOrDefaultString("FILE_STORAGE_PATH", *flagFileStorage)
	databaseDSN := util.GetEnvOrDefaultString("DATABASE_DSN", *flagDatabaseDSN)

	storeInterval, err := util.GetEnvOrDefault("STORE_INTERVAL", *flagStoreInterval, strconv.Atoi)
	if err != nil {
		logger.Sugar.Fatalln(err.Error())
	}

	restore, err := util.GetEnvOrDefault("RESTORE", *flagRestore, strconv.ParseBool)
	if err != nil {
		logger.Sugar.Fatalln(err.Error())
	}

	database, err := db.NewPostgres(databaseDSN)
	if err != nil {
		logger.Sugar.Fatalln(err.Error())
	}
	if database != nil {
		defer database.Close()
	}

	repo, memStorage := newRepository(database, fileStoragePath, storeInterval, restore)

	svc := service.NewMetricsService(repo)
	h := handler.NewMetricsHandler(svc)
	pingHandler := handler.NewPingHandler(database)

	r := chi.NewRouter()
	r.Use(logger.WithLogging)
	r.Use(middleware.Gzip)
	r.Use(chiMiddleware.StripSlashes)
	r.Get("/", h.Index)
	r.Get("/ping", pingHandler.Ping)
	r.Post("/update", h.UpdateJSON)
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Post("/value", h.ValueJSON)
	r.Get("/value/{type}/{name}", h.Value)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if memStorage != nil && fileStoragePath != "" && storeInterval > 0 {
		go storePeriodically(ctx, memStorage, fileStoragePath, time.Duration(storeInterval)*time.Second)
	}

	server := &http.Server{Addr: addr, Handler: r}

	go func() {
		logger.Sugar.Infow("starting server",
			"addr", addr,
			"store_interval", storeInterval,
			"file_storage_path", fileStoragePath,
			"restore", restore,
			"database_configured", database != nil,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal(err.Error())
		}
	}()

	<-ctx.Done()
	logger.Log.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Sugar.Errorw("server shutdown error", "error", err)
	}

	if memStorage != nil && fileStoragePath != "" {
		if err := memStorage.SaveToFile(fileStoragePath); err != nil {
			logger.Sugar.Errorw("failed to save metrics on shutdown", "error", err)
		} else {
			logger.Sugar.Infow("metrics saved", "path", fileStoragePath)
		}
	}
}

func newRepository(database *sql.DB, fileStoragePath string, storeInterval int, restore bool) (repository.Repository, *repository.MemStorage) {
	if database != nil {
		logger.Sugar.Info("using postgres storage")
		return repository.NewPostgresStorage(database), nil
	}

	if fileStoragePath != "" {
		storage := repository.NewMemStorage()
		if restore {
			if err := storage.LoadFromFile(fileStoragePath); err != nil {
				logger.Sugar.Errorw("failed to restore metrics", "error", err)
			} else {
				logger.Sugar.Infow("metrics restored", "path", fileStoragePath)
			}
		}
		if storeInterval == 0 {
			return repository.NewSyncFileStorage(storage, fileStoragePath), storage
		}
		return storage, storage
	}

	logger.Sugar.Info("using memory storage")
	storage := repository.NewMemStorage()
	return storage, storage
}

func storePeriodically(ctx context.Context, storage *repository.MemStorage, path string, interval time.Duration) {
	logger.Log.Info("start async storage saving job")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := storage.SaveToFile(path); err != nil {
				logger.Sugar.Errorw("failed to save metrics", "error", err)
			} else {
				logger.Sugar.Infof("saved metrics to %s", path)
			}
		case <-ctx.Done():
			return
		}
	}
}

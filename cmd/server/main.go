package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
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
)

func main() {
	logger.Initialize("INFO")
	defer logger.Log.Sync()

	flag.Parse()

	addr := util.GetEnvOrDefaultString("ADDRESS", *flagAddr)
	storeInterval := util.GetEnvOrDefault("STORE_INTERVAL", *flagStoreInterval, strconv.Atoi)
	fileStoragePath := util.GetEnvOrDefaultString("FILE_STORAGE_PATH", *flagFileStorage)
	restore := util.GetEnvOrDefault("RESTORE", *flagRestore, strconv.ParseBool)

	storage := repository.NewMemStorage()

	if restore {
		if err := storage.LoadFromFile(fileStoragePath); err != nil {
			logger.Sugar.Errorw("failed to restore metrics", "error", err)
		} else {
			logger.Sugar.Infow("metrics restored", "path", fileStoragePath)
		}
	}

	var repo repository.Repository = storage
	if storeInterval == 0 {
		repo = repository.NewSyncFileStorage(storage, fileStoragePath)
	}

	svc := service.NewMetricsService(repo)
	h := handler.NewMetricsHandler(svc)

	r := chi.NewRouter()
	r.Use(logger.WithLogging)
	r.Use(middleware.Gzip)
	r.Use(chiMiddleware.StripSlashes)
	r.Get("/", h.Index)
	r.Post("/update", h.UpdateJSON)
	r.Post("/update/{type}/{name}/{value}", h.Update)
	r.Post("/value", h.ValueJSON)
	r.Get("/value/{type}/{name}", h.Value)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if storeInterval > 0 {
		go storePeriodically(ctx, storage, fileStoragePath, time.Duration(storeInterval)*time.Second)
	}

	server := &http.Server{Addr: addr, Handler: r}

	go func() {
		logger.Sugar.Infow("starting server",
			"addr", addr,
			"store_interval", storeInterval,
			"file_storage_path", fileStoragePath,
			"restore", restore,
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

	if err := storage.SaveToFile(fileStoragePath); err != nil {
		logger.Sugar.Errorw("failed to save metrics on shutdown", "error", err)
	} else {
		logger.Sugar.Infow("metrics saved", "path", fileStoragePath)
	}
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

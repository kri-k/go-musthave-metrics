package repository

import "github.com/kri-k/go-musthave-metrics/internal/logger"

type SyncFileStorage struct {
	*MemStorage
	path string
}

func NewSyncFileStorage(storage *MemStorage, path string) *SyncFileStorage {
	return &SyncFileStorage{MemStorage: storage, path: path}
}

func (s *SyncFileStorage) UpdateGauge(name string, value float64) float64 {
	v := s.MemStorage.UpdateGauge(name, value)
	if err := s.MemStorage.SaveToFile(s.path); err != nil {
		logger.Sugar.Errorw("failed to save metrics", "error", err)
	}
	return v
}

func (s *SyncFileStorage) UpdateCounter(name string, value int64) int64 {
	v := s.MemStorage.UpdateCounter(name, value)
	if err := s.MemStorage.SaveToFile(s.path); err != nil {
		logger.Sugar.Errorw("failed to save metrics", "error", err)
	}
	return v
}

var _ Repository = (*SyncFileStorage)(nil)

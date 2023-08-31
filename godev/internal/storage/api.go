package storage

import (
	"context"

	"golang.org/x/telemetry/godev/internal/config"
)

type API struct {
	Upload BucketHandle
	Merge  BucketHandle
	Chart  BucketHandle
}

func NewAPI(ctx context.Context, cfg *config.Config) (*API, error) {
	upload, err := newBucket(ctx, cfg, cfg.UploadBucket)
	if err != nil {
		return nil, err
	}
	merge, err := newBucket(ctx, cfg, cfg.MergedBucket)
	if err != nil {
		return nil, err
	}
	chart, err := newBucket(ctx, cfg, cfg.ChartDataBucket)
	if err != nil {
		return nil, err
	}
	return &API{upload, merge, chart}, nil
}

func newBucket(ctx context.Context, cfg *config.Config, name string) (BucketHandle, error) {
	if cfg.UseGCS {
		return NewGCSBucket(ctx, cfg.ProjectID, name)
	}
	return NewFSBucket(ctx, cfg.LocalStorage, name)
}

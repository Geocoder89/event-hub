package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)


func NewPool (dbURL string) (*pgxpool.Pool,error){
	cfg, err := pgxpool.ParseConfig(dbURL)

	if err != nil {
		return nil, err
	}

	cfg.MaxConns = 5

	ctx,cancel := context.WithTimeout(context.Background(), 5 * time.Second)

	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx,cfg)

	if err != nil {
		return nil, err
	}

	err = pool.Ping(ctx)

	if err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
package redisclient

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	redisdb *redis.Client
}

type Config struct {
	Addr     string
	Password string
	DB       int
}

func New(cfg Config) *Client {
	redisdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	return &Client{redisdb: redisdb}
}

// this ping function checks redis connectivity

func (c *Client) Ping(ctx context.Context) error {
	return c.redisdb.Ping(ctx).Err()
}

// this closes the client

func (c *Client) Close() error {
	return c.redisdb.Close()
}

//  this exposes the redis client for later days (producer/ worker flow)

func (c *Client) Raw() *redis.Client {
	return c.redisdb
}

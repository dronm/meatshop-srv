package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dronm/meatshop/internal/config"
	"github.com/dronm/meatshop/internal/models"
	"github.com/redis/go-redis/v9"
)

var ErrMainMenuCacheMiss = errors.New("main menu cache miss")

type MainMenuCache interface {
	Get(ctx context.Context, userID int, roleID models.RoleID) ([]*models.MainMenuForUser, int64, error)
	Set(ctx context.Context, userID int, roleID models.RoleID, version int64, menu []*models.MainMenuForUser) error
	Invalidate(ctx context.Context) error
	Close() error
}

type mainMenuRedisCache struct {
	client     *redis.Client
	keyPrefix  string
	versionKey string
	ttl        time.Duration
}

func NewMainMenuRedisCache(
	ctx context.Context,
	redisCfg config.RedisConfig,
	ttl time.Duration,
) (MainMenuCache, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("main menu cache ttl should be positive")
	}

	client, err := newRedisClient(redisCfg.Connect)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis for main menu cache: %w", err)
	}

	namespace := strings.Trim(strings.TrimSpace(redisCfg.Namespace), ":")
	if namespace == "" {
		namespace = "steklomarket"
	}

	keyPrefix := namespace + ":cache:main-menu"
	cache := &mainMenuRedisCache{
		client:     client,
		keyPrefix:  keyPrefix,
		versionKey: keyPrefix + ":version",
		ttl:        ttl,
	}

	if err := cache.client.SetNX(ctx, cache.versionKey, 1, 0).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("initialize main menu cache version: %w", err)
	}

	return cache, nil
}

func newRedisClient(connect string) (*redis.Client, error) {
	connect = strings.TrimSpace(connect)
	if connect == "" {
		return nil, fmt.Errorf("redis connection is required")
	}

	if strings.Contains(connect, "://") {
		options, err := redis.ParseURL(connect)
		if err != nil {
			return nil, fmt.Errorf("parse redis connection: %w", err)
		}
		return redis.NewClient(options), nil
	}

	return redis.NewClient(&redis.Options{Addr: connect}), nil
}

func (c *mainMenuRedisCache) Get(
	ctx context.Context,
	userID int,
	roleID models.RoleID,
) ([]*models.MainMenuForUser, int64, error) {
	version, err := c.version(ctx)
	if err != nil {
		return nil, 0, err
	}

	body, err := c.client.Get(ctx, c.menuKey(version, userID, roleID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, version, ErrMainMenuCacheMiss
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read main menu cache: %w", err)
	}

	menu := make([]*models.MainMenuForUser, 0)
	if err := json.Unmarshal(body, &menu); err != nil {
		return nil, 0, fmt.Errorf("decode main menu cache: %w", err)
	}

	return menu, version, nil
}

func (c *mainMenuRedisCache) Set(
	ctx context.Context,
	userID int,
	roleID models.RoleID,
	version int64,
	menu []*models.MainMenuForUser,
) error {
	if version <= 0 {
		return nil
	}

	currentVersion, err := c.version(ctx)
	if err != nil {
		return err
	}
	if currentVersion != version {
		return nil
	}

	body, err := json.Marshal(menu)
	if err != nil {
		return fmt.Errorf("encode main menu cache: %w", err)
	}

	if err := c.client.Set(ctx, c.menuKey(version, userID, roleID), body, c.ttl).Err(); err != nil {
		return fmt.Errorf("write main menu cache: %w", err)
	}

	return nil
}

func (c *mainMenuRedisCache) Invalidate(ctx context.Context) error {
	if err := c.client.Incr(ctx, c.versionKey).Err(); err != nil {
		return fmt.Errorf("increment main menu cache version: %w", err)
	}
	return nil
}

func (c *mainMenuRedisCache) Close() error {
	return c.client.Close()
}

func (c *mainMenuRedisCache) version(ctx context.Context) (int64, error) {
	value, err := c.client.Get(ctx, c.versionKey).Result()
	if errors.Is(err, redis.Nil) {
		if err := c.client.Set(ctx, c.versionKey, 1, 0).Err(); err != nil {
			return 0, fmt.Errorf("restore main menu cache version: %w", err)
		}
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read main menu cache version: %w", err)
	}

	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse main menu cache version: %w", err)
	}

	return version, nil
}

func (c *mainMenuRedisCache) menuKey(version int64, userID int, roleID models.RoleID) string {
	return fmt.Sprintf("%s:v%d:user:%d:role:%s", c.keyPrefix, version, userID, roleID)
}

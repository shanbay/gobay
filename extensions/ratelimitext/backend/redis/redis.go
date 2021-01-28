package redis

import (
	"context"

	redis "github.com/go-redis/redis/v8"
	"github.com/spf13/viper"

	"github.com/shanbay/gobay/extensions/ratelimitext"

	redis_rate "github.com/go-redis/redis_rate/v9"
)

func init() {
	if err := ratelimitext.RegisteBackend("redis", func() ratelimitext.RatelimitBackend { return &redisBackend{} }); err != nil {
		panic("RedisBackend init error")
	}
}

type redisBackend struct {
	client  *redis.Client
	limiter *redis_rate.Limiter
}

func (b *redisBackend) withContext(ctx context.Context) *redis.Client {
	return b.client.WithContext(ctx)
}

func (b *redisBackend) Init(config *viper.Viper) error {
	host := config.GetString("host")
	password := config.GetString("password")
	dbNum := config.GetInt("db")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       dbNum,
	})
	b.client = redisClient
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	b.limiter = redis_rate.NewLimiter(redisClient)
	return err
}

func (b *redisBackend) CheckHealth(ctx context.Context) error {
	client := b.withContext(ctx)
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}

func (b *redisBackend) Allow(ctx context.Context, key string, limitAmount, limitBaseSeconds int) (allowed int, remaining int, err error) {
	var limit redis_rate.Limit
	if limitBaseSeconds == 1 {
		limit = redis_rate.PerSecond(limitAmount)
	} else if limitBaseSeconds == 60 {
		limit = redis_rate.PerMinute(limitAmount)
	} else if limitBaseSeconds == 3600 {
		limit = redis_rate.PerHour(limitAmount)
	} else {
		panic("invalid limitBaseSeconds, pick one of [1,60,3600] plz")
	}

	res, err := b.limiter.Allow(ctx, key, limit)
	if err != nil {
		return 1, 0, err
	}
	return res.Allowed, res.Remaining, nil
}

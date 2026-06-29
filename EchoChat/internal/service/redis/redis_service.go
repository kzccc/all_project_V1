package redis

// 本文件封装 Redis 的基础读写、模糊删除和全量清理能力，供各业务模块复用。

import (
	"context"
	"echo_chat_server/internal/config"
	"echo_chat_server/internal/observability"
	"echo_chat_server/pkg/zlog"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"strconv"
	"time"
)

// redisClient 是全局复用的 Redis 客户端。
var redisClient *redis.Client

// ctx 复用后台 context，适合当前简单的阻塞式 Redis 访问场景。
var ctx = context.Background()

var initFloorAndIncrScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current then
  redis.call("SET", KEYS[1], ARGV[1])
end
return redis.call("INCR", KEYS[1])
`)

var ensureMinAndIncrScript = redis.NewScript(`
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
local floor = tonumber(ARGV[1])
if current < floor then
  redis.call("SET", KEYS[1], floor)
end
return redis.call("INCR", KEYS[1])
`)

var delKeyIfValueMatchesScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current then
  return 0
end
if current ~= ARGV[1] then
  return 0
end
return redis.call("DEL", KEYS[1])
`)

// init 根据配置文件初始化 Redis 连接。
func init() {
	conf := config.GetConfig()
	host := conf.RedisConfig.Host
	port := conf.RedisConfig.Port
	password := conf.RedisConfig.Password
	db := conf.Db
	addr := host + ":" + strconv.Itoa(port)

	redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// SetClientForTest 仅用于测试阶段临时替换全局 Redis 客户端。
func SetClientForTest(client *redis.Client) func() {
	previous := redisClient
	redisClient = client
	return func() {
		redisClient = previous
	}
}

// SetKeyEx 写入带过期时间的字符串键值。
func SetKeyEx(key string, value string, timeout time.Duration) error {
	start := time.Now()
	err := redisClient.Set(ctx, key, value, timeout).Err()
	result := "success"
	if err != nil {
		result = "failure"
		observability.ObserveRedisCommand("set", time.Since(start), result)
		return err
	}
	observability.ObserveRedisCommand("set", time.Since(start), result)
	return nil
}

// SetKey 写入一个不过期的字符串键值。
func SetKey(key string, value string) error {
	start := time.Now()
	err := redisClient.Set(ctx, key, value, 0).Err()
	result := "success"
	if err != nil {
		result = "failure"
		observability.ObserveRedisCommand("set", time.Since(start), result)
		return err
	}
	observability.ObserveRedisCommand("set", time.Since(start), result)
	return nil
}

// SetNXAndIncr 在键不存在时先把计数器初始化到指定 floor，然后执行一次自增并返回结果。
func SetNXAndIncr(key string, floor int64) (int64, error) {
	setnxStart := time.Now()
	if _, err := redisClient.SetNX(ctx, key, floor, 0).Result(); err != nil {
		observability.ObserveRedisCommand("setnx", time.Since(setnxStart), "failure")
		return 0, err
	}
	observability.ObserveRedisCommand("setnx", time.Since(setnxStart), "success")
	incrStart := time.Now()
	value, err := redisClient.Incr(ctx, key).Result()
	result := "success"
	if err != nil {
		result = "failure"
		observability.ObserveRedisCommand("incr", time.Since(incrStart), result)
		return 0, err
	}
	observability.ObserveRedisCommand("incr", time.Since(incrStart), result)
	return value, nil
}

// InitFloorAndIncr 使用 Lua 脚本在 key 不存在时写入 floor，再执行一次自增。
func InitFloorAndIncr(key string, floor int64) (int64, error) {
	start := time.Now()
	value, err := initFloorAndIncrScript.Run(ctx, redisClient, []string{key}, floor).Int64()
	result := "success"
	if err != nil {
		result = "failure"
	}
	observability.ObserveRedisCommand("eval_init_floor_incr", time.Since(start), result)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// EnsureMinAndIncr 在 key 已存在但值过小时先抬到 floor，再执行一次自增。
func EnsureMinAndIncr(key string, floor int64) (int64, error) {
	start := time.Now()
	value, err := ensureMinAndIncrScript.Run(ctx, redisClient, []string{key}, floor).Int64()
	result := "success"
	if err != nil {
		result = "failure"
	}
	observability.ObserveRedisCommand("eval_ensure_min_incr", time.Since(start), result)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// IncrKey 对现有计数器做一次原子递增。
func IncrKey(key string) (int64, error) {
	incrStart := time.Now()
	value, err := redisClient.Incr(ctx, key).Result()
	result := "success"
	if err != nil {
		result = "failure"
	}
	observability.ObserveRedisCommand("incr", time.Since(incrStart), result)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// GetKey 读取指定 key；若 key 不存在则返回空字符串和空错误。
func GetKey(key string) (string, error) {
	start := time.Now()
	value, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			observability.ObserveRedisCommand("get", time.Since(start), "not_found")
			zlog.Info("该key不存在")
			return "", nil
		}
		observability.ObserveRedisCommand("get", time.Since(start), "failure")
		return "", err
	}
	observability.ObserveRedisCommand("get", time.Since(start), "success")
	return value, nil
}

// GetKeyNilIsErr 读取指定 key，并把 redis.Nil 原样抛给上层判断。
func GetKeyNilIsErr(key string) (string, error) {
	start := time.Now()
	value, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			observability.ObserveRedisCommand("get", time.Since(start), "not_found")
		} else {
			observability.ObserveRedisCommand("get", time.Since(start), "failure")
		}
		return "", err
	}
	observability.ObserveRedisCommand("get", time.Since(start), "success")
	return value, nil
}

// GetKeyWithPrefixNilIsErr 按前缀匹配唯一键，常用于 session 这类带随机后缀的缓存。
func GetKeyWithPrefixNilIsErr(prefix string) (string, error) {
	var keys []string
	var err error

	for {
		// 当前实现直接用 Keys，简单直观，但在大 keyspace 下并不高效。
		keys, err = redisClient.Keys(ctx, prefix+"*").Result()
		if err != nil {
			return "", err
		}

		if len(keys) == 0 {
			zlog.Info("没有找到相关前缀key")
			return "", redis.Nil
		}

		if len(keys) == 1 {
			zlog.Info(fmt.Sprintln("成功找到了相关前缀key", keys))
			return keys[0], nil
		} else {
			zlog.Error("找到了数量大于1的key，查找异常")
			return "", errors.New("找到了数量大于1的key，查找异常")
		}
	}

}

// GetKeyWithSuffixNilIsErr 按后缀匹配唯一键，适用于反向定位场景。
func GetKeyWithSuffixNilIsErr(suffix string) (string, error) {
	var keys []string
	var err error

	for {
		// 使用 Keys 命令迭代匹配的键
		keys, err = redisClient.Keys(ctx, "*"+suffix).Result()
		if err != nil {
			return "", err
		}

		if len(keys) == 0 {
			zlog.Info("没有找到相关后缀key")
			return "", redis.Nil
		}

		if len(keys) == 1 {
			zlog.Info(fmt.Sprintln("成功找到了相关后缀key", keys))
			return keys[0], nil
		} else {
			zlog.Error("找到了数量大于1的key，查找异常")
			return "", errors.New("找到了数量大于1的key，查找异常")
		}
	}

}

// DelKeyIfExists 仅在 key 存在时删除，避免把“不存在”视作异常。
func DelKeyIfExists(key string) error {
	exists, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 1 { // 键存在
		delErr := redisClient.Del(ctx, key).Err()
		if delErr != nil {
			return delErr
		}
	}
	// 无论键是否存在，都不返回错误
	return nil
}

// KeyExists 返回 key 是否存在。
func KeyExists(key string) (bool, error) {
	start := time.Now()
	exists, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		observability.ObserveRedisCommand("exists", time.Since(start), "failure")
		return false, err
	}
	observability.ObserveRedisCommand("exists", time.Since(start), "success")
	return exists > 0, nil
}

// DelKeyIfValueMatches 仅在 key 当前值与 expectedValue 相等时删除，避免误删被其他实例接管后的路由。
func DelKeyIfValueMatches(key string, expectedValue string) error {
	start := time.Now()
	_, err := delKeyIfValueMatchesScript.Run(ctx, redisClient, []string{key}, expectedValue).Int()
	result := "success"
	if err != nil {
		if errors.Is(err, redis.Nil) {
			result = "not_found"
		} else {
			result = "failure"
		}
	}
	observability.ObserveRedisCommand("eval_compare_del", time.Since(start), result)
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	return nil
}

// Publish 把消息发布到指定 Redis Pub/Sub channel。
func Publish(channel string, payload string) error {
	start := time.Now()
	err := redisClient.Publish(ctx, channel, payload).Err()
	result := "success"
	if err != nil {
		result = "failure"
	}
	observability.ObserveRedisCommand("publish", time.Since(start), result)
	return err
}

// Subscribe 订阅一个或多个 Redis Pub/Sub channel。
func Subscribe(channels ...string) *redis.PubSub {
	return redisClient.Subscribe(ctx, channels...)
}

// DelKeysWithPattern 按完整模式删除一批 key。
func DelKeysWithPattern(pattern string) error {
	var keys []string
	var err error

	for {
		// 使用 Keys 命令迭代匹配的键
		keys, err = redisClient.Keys(ctx, pattern).Result()
		if err != nil {
			return err
		}

		// 如果没有更多的键，则跳出循环
		if len(keys) == 0 {
			zlog.Info(
				"redis.delete.pattern.no_keys",
				zap.String("event", "redis.delete.pattern.no_keys"),
				zap.String("pattern", pattern),
			)
			break
		}

		// 删除找到的键
		if len(keys) > 0 {
			_, err = redisClient.Del(ctx, keys...).Result()
			if err != nil {
				return err
			}
			zlog.Info(
				"redis.delete.pattern.success",
				zap.String("event", "redis.delete.pattern.success"),
				zap.String("pattern", pattern),
				zap.Int("count", len(keys)),
			)
		}
	}

	return nil
}

// DelKeysWithPrefix 删除指定前缀下的全部 key。
func DelKeysWithPrefix(prefix string) error {
	//var cursor uint64 = 0
	var keys []string
	var err error

	for {
		// 使用 Keys 命令迭代匹配的键
		keys, err = redisClient.Keys(ctx, prefix+"*").Result()
		if err != nil {
			return err
		}

		// 如果没有更多的键，则跳出循环
		if len(keys) == 0 {
			zlog.Info(
				"redis.delete.prefix.no_keys",
				zap.String("event", "redis.delete.prefix.no_keys"),
				zap.String("prefix", prefix),
			)
			break
		}

		// 删除找到的键
		if len(keys) > 0 {
			_, err = redisClient.Del(ctx, keys...).Result()
			if err != nil {
				return err
			}
			zlog.Info(
				"redis.delete.prefix.success",
				zap.String("event", "redis.delete.prefix.success"),
				zap.String("prefix", prefix),
				zap.Int("count", len(keys)),
			)
		}
	}

	return nil
}

// DelKeysWithSuffix 删除指定后缀下的全部 key。
func DelKeysWithSuffix(suffix string) error {
	//var cursor uint64 = 0
	var keys []string
	var err error

	for {
		// 使用 Keys 命令迭代匹配的键
		keys, err = redisClient.Keys(ctx, "*"+suffix).Result()
		if err != nil {
			return err
		}

		// 如果没有更多的键，则跳出循环
		if len(keys) == 0 {
			zlog.Info(
				"redis.delete.suffix.no_keys",
				zap.String("event", "redis.delete.suffix.no_keys"),
				zap.String("suffix", suffix),
			)
			break
		}

		// 删除找到的键
		if len(keys) > 0 {
			_, err = redisClient.Del(ctx, keys...).Result()
			if err != nil {
				return err
			}
			zlog.Info(
				"redis.delete.suffix.success",
				zap.String("event", "redis.delete.suffix.success"),
				zap.String("suffix", suffix),
				zap.Int("count", len(keys)),
			)
		}
	}

	return nil
}

// DeleteAllRedisKeys 清空当前 Redis 逻辑库中的全部 key，主要用于服务退出时收尾。
func DeleteAllRedisKeys() error {
	var cursor uint64 = 0
	for {
		keys, nextCursor, err := redisClient.Scan(ctx, cursor, "*", 0).Result()
		if err != nil {
			return err
		}
		cursor = nextCursor

		if len(keys) > 0 {
			_, err := redisClient.Del(ctx, keys...).Result()
			if err != nil {
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}
	return nil
}

// Close 关闭全局 Redis 客户端连接，供服务退出时释放底层网络资源。
func Close() error {
	if redisClient == nil {
		return nil
	}
	return redisClient.Close()
}

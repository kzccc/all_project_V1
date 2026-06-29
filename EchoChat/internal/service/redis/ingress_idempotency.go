package redis

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"echo_chat_server/internal/observability"
)

type IngressIdempotencyStatus string

const (
	IngressIdempotencyStatusPending IngressIdempotencyStatus = "PENDING"
	IngressIdempotencyStatusDone    IngressIdempotencyStatus = "DONE"
)

type IngressIdempotencyRecord struct {
	Status  IngressIdempotencyStatus
	Payload string
}

var tryAcquireIngressIdempotencyScript = redis.NewScript(`
local status = redis.call("HGET", KEYS[1], "status")
if not status or status == "" or (status ~= "PENDING" and status ~= "DONE") then
  redis.call("DEL", KEYS[1])
  redis.call("HSET", KEYS[1], "status", "PENDING", "owner", ARGV[1])
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
  return {"ACQUIRED", "", ""}
end
local payload = redis.call("HGET", KEYS[1], "payload") or ""
return {"EXISTS", status, payload}
`)

var markIngressIdempotencyDoneScript = redis.NewScript(`
local status = redis.call("HGET", KEYS[1], "status")
local owner = redis.call("HGET", KEYS[1], "owner")
if status ~= "PENDING" then
  return 0
end
if ARGV[1] ~= "" and owner ~= ARGV[1] then
  return 0
end
redis.call("HSET", KEYS[1], "status", "DONE", "payload", ARGV[2])
redis.call("HDEL", KEYS[1], "owner")
redis.call("PEXPIRE", KEYS[1], ARGV[3])
return 1
`)

var clearIngressIdempotencyPendingScript = redis.NewScript(`
local status = redis.call("HGET", KEYS[1], "status")
local owner = redis.call("HGET", KEYS[1], "owner")
if status ~= "PENDING" then
  return 0
end
if ARGV[1] ~= "" and owner ~= ARGV[1] then
  return 0
end
return redis.call("DEL", KEYS[1])
`)

// TryAcquireIngressIdempotency 尝试为发送入口抢占幂等 key。
func TryAcquireIngressIdempotency(key string, owner string, pendingTTL time.Duration) (bool, IngressIdempotencyRecord, error) {
	start := time.Now()
	result, err := tryAcquireIngressIdempotencyScript.Run(
		ctx,
		redisClient,
		[]string{key},
		owner,
		pendingTTL.Milliseconds(),
	).Result()
	commandResult := "success"
	if err != nil {
		commandResult = "failure"
		observability.ObserveRedisCommand("eval_ingress_try_acquire", time.Since(start), commandResult)
		return false, IngressIdempotencyRecord{}, err
	}
	observability.ObserveRedisCommand("eval_ingress_try_acquire", time.Since(start), commandResult)

	items, ok := result.([]interface{})
	if !ok || len(items) < 3 {
		return false, IngressIdempotencyRecord{}, fmt.Errorf("unexpected ingress idempotency lua result: %#v", result)
	}
	mode, err := luaResultString(items[0])
	if err != nil {
		return false, IngressIdempotencyRecord{}, err
	}
	status, err := luaResultString(items[1])
	if err != nil {
		return false, IngressIdempotencyRecord{}, err
	}
	payload, err := luaResultString(items[2])
	if err != nil {
		return false, IngressIdempotencyRecord{}, err
	}
	if mode == "ACQUIRED" {
		return true, IngressIdempotencyRecord{}, nil
	}
	return false, IngressIdempotencyRecord{
		Status:  IngressIdempotencyStatus(status),
		Payload: payload,
	}, nil
}

// MarkIngressIdempotencyDone 把入口幂等状态从 PENDING 推进到 DONE，并保存可回放结果。
func MarkIngressIdempotencyDone(key string, owner string, payload string, doneTTL time.Duration) (bool, error) {
	start := time.Now()
	updated, err := markIngressIdempotencyDoneScript.Run(
		ctx,
		redisClient,
		[]string{key},
		owner,
		payload,
		doneTTL.Milliseconds(),
	).Int()
	commandResult := "success"
	if err != nil {
		commandResult = "failure"
	}
	observability.ObserveRedisCommand("eval_ingress_mark_done", time.Since(start), commandResult)
	if err != nil {
		return false, err
	}
	return updated > 0, nil
}

// ClearIngressIdempotencyPending 仅在当前请求仍持有 PENDING 锁时删除幂等 key。
func ClearIngressIdempotencyPending(key string, owner string) (bool, error) {
	start := time.Now()
	deleted, err := clearIngressIdempotencyPendingScript.Run(
		ctx,
		redisClient,
		[]string{key},
		owner,
	).Int()
	commandResult := "success"
	if err != nil {
		commandResult = "failure"
	}
	observability.ObserveRedisCommand("eval_ingress_clear_pending", time.Since(start), commandResult)
	if err != nil {
		return false, err
	}
	return deleted > 0, nil
}

func luaResultString(value interface{}) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		return "", fmt.Errorf("unexpected lua result type %T", value)
	}
}

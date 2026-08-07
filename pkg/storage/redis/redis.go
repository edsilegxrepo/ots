// Package redis implements a Redis backed storage for secrets
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	redis "github.com/redis/go-redis/v9"

	"github.com/Luzifer/ots/pkg/storage"
)

const (
	redisDefaultPrefix = "io.luzifer.ots"
	redisScanCount     = 10
)

type (
	storageRedis struct {
		conn *redis.Client
	}

	redisPayload struct {
		Secret         string `json:"secret"`
		ReadsRemaining int    `json:"reads_remaining"`
	}
)

var luaReadAndDestroy = redis.NewScript(`
local key = KEYS[1]
local data = redis.call("GET", key)
if not data then
    return nil
end

local secret = data
local reads_remaining = 0

if string.sub(data, 1, 1) == "{" then
    local ok, secretObj = pcall(cjson.decode, data)
    if ok and secretObj and secretObj.secret then
        secret = secretObj.secret
        reads_remaining = (secretObj.reads_remaining or 1) - 1
    end
if reads_remaining <= 0 then
    redis.call("DEL", key)
else
    local secretObj = cjson.decode(data)
    secretObj.reads_remaining = reads_remaining
    local ttl = redis.call("TTL", key)
    if ttl > 0 then
        redis.call("SET", key, cjson.encode(secretObj), "EX", ttl)
    else
        redis.call("SET", key, cjson.encode(secretObj))
    end
return { secret, reads_remaining }
`)

// New returns a new Redis backed storage
func New() (storage.Storage, error) {
	if os.Getenv("REDIS_URL") == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable not set")
	}

	// We replace the old URI format
	//		tcp://auth:password@127.0.0.1:6379/0
	// with the new one
	//		redis://<user>:<password>@<host>:<port>/<db_number>
	// in order to maintain backwards compatibility
	opt, err := redis.ParseURL(strings.Replace(os.Getenv("REDIS_URL"), "tcp://", "redis://", 1))
	if err != nil {
		return nil, fmt.Errorf("parsing REDIS_URL: %w", err)
	}

	s := &storageRedis{
		conn: redis.NewClient(opt),
	}

	return s, nil
}

func (s storageRedis) Count() (n int64, err error) {
	var cursor uint64

	for {
		var keys []string

		keys, cursor, err = s.conn.Scan(context.Background(), cursor, s.redisKey("*"), redisScanCount).Result()
		if err != nil {
			return n, fmt.Errorf("scanning stored keys: %w", err)
		}

		n += int64(len(keys))
		if cursor == 0 {
			break
		}
	}

	return n, nil
}

func (s storageRedis) Create(secret string, expireIn time.Duration, reads int) (string, error) {
	if reads <= 0 {
		reads = 1
	}

	id := uuid.Must(uuid.NewV4()).String()
	// #nosec G117 -- False positive: redisPayload.Secret represents the encrypted ciphertext blob being stored by design in Redis
	payload, err := json.Marshal(redisPayload{
		Secret:         secret,
		ReadsRemaining: reads,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling redis payload: %w", err)
	}

	err = s.conn.Set(context.Background(), s.redisKey(id), string(payload), expireIn).Err()
	if err != nil {
		return "", fmt.Errorf("writing redis key: %w", err)
	}

	return id, nil
}

func (s storageRedis) ReadAndDestroy(id string) (string, int, error) {
	res, err := luaReadAndDestroy.Run(context.Background(), s.conn, []string{s.redisKey(id)}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", 0, storage.ErrSecretNotFound
		}
		return "", 0, fmt.Errorf("getting and updating redis key: %w", err)
	}

	arr, ok := res.([]any)
	if !ok || len(arr) < 2 {
		return "", 0, storage.ErrSecretNotFound
	}

	secStr, _ := arr[0].(string)
	readsRem, _ := arr[1].(int64)

	return secStr, int(readsRem), nil
}

func (storageRedis) redisKey(id string) string {
	prefix := redisDefaultPrefix
	if prfx := os.Getenv("REDIS_KEY"); prfx != "" {
		prefix = prfx
	}

	return strings.Join([]string{prefix, id}, ":")
}

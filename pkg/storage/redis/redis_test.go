// Package redis - Redis Storage Engine Unit Test Suite
package redis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

type mockRedisServer struct {
	listener net.Listener
	mu       sync.Mutex
	kv       map[string]string
	quit     chan struct{}
}

func startMockRedisServer(t *testing.T) (*mockRedisServer, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &mockRedisServer{
		listener: l,
		kv:       make(map[string]string),
		quit:     make(chan struct{}),
	}

	go srv.serve()

	addr := fmt.Sprintf("redis://%s/0", l.Addr().String())
	return srv, addr
}

func (s *mockRedisServer) Close() {
	close(s.quit)
	_ = s.listener.Close()
}

func (s *mockRedisServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				return
			}
		}
		go s.handleConn(conn)
	}
}

func (s *mockRedisServer) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "*") {
			continue
		}

		var count int
		fmt.Sscanf(line, "*%d", &count)
		args := make([]string, 0, count)
		for i := 0; i < count; i++ {
			lenLine, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			var argLen int
			fmt.Sscanf(strings.TrimSpace(lenLine), "$%d", &argLen)
			buf := make([]byte, argLen+2)
			_, _ = io.ReadFull(reader, buf)
			args = append(args, string(buf[:argLen]))
		}

		if len(args) == 0 {
			continue
		}

		cmd := strings.ToUpper(args[0])
		s.mu.Lock()
		switch cmd {
		case "PING":
			_, _ = conn.Write([]byte("+PONG\r\n"))
		case "HELLO":
			_, _ = conn.Write([]byte("%1\r\n$5\r\nproto\r\n:2\r\n"))
		case "COMMAND":
			_, _ = conn.Write([]byte("*0\r\n"))
		case "SET":
			if len(args) >= 3 {
				s.kv[args[1]] = args[2]
			}
			_, _ = conn.Write([]byte("+OK\r\n"))
		case "GET":
			if val, ok := s.kv[args[1]]; ok {
				_, _ = fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(val), val)
			} else {
				_, _ = conn.Write([]byte("$-1\r\n"))
			}
		case "DEL":
			deleted := 0
			for _, k := range args[1:] {
				if _, ok := s.kv[k]; ok {
					delete(s.kv, k)
					deleted++
				}
			}
			_, _ = fmt.Fprintf(conn, ":%d\r\n", deleted)
		case "SCAN":
			var keys []string
			for k := range s.kv {
				keys = append(keys, k)
			}
			resp := fmt.Sprintf("*2\r\n$1\r\n0\r\n*%d\r\n", len(keys))
			for _, k := range keys {
				resp += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
			}
			_, _ = conn.Write([]byte(resp))
		case "SCRIPT":
			_, _ = conn.Write([]byte("$40\r\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4\r\n"))
		case "EVAL", "EVALSHA":
			if len(args) >= 4 {
				key := args[3]
				if val, ok := s.kv[key]; ok {
					delete(s.kv, key)
					var p struct {
						Payload []byte `json:"p"`
					}
					sec := val
					if err := json.Unmarshal([]byte(val), &p); err == nil && len(p.Payload) > 0 {
						sec = string(p.Payload)
					}
					resp := fmt.Sprintf("*2\r\n$%d\r\n%s\r\n:0\r\n", len(sec), sec)
					_, _ = conn.Write([]byte(resp))
				} else {
					_, _ = conn.Write([]byte("$-1\r\n"))
				}
			} else {
				_, _ = conn.Write([]byte("$-1\r\n"))
			}
		default:
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
		s.mu.Unlock()
	}
}

func TestRedisStorageNewValidation(t *testing.T) {
	// Test missing REDIS_URL
	t.Setenv("REDIS_URL", "")
	store, err := New()
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "REDIS_URL environment variable not set")

	// Test invalid REDIS_URL
	t.Setenv("REDIS_URL", "invalid_scheme://[::1]:namedport")
	store, err = New()
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "parsing REDIS_URL")
}

func TestRedisKeyFormatting(t *testing.T) {
	s := storageRedis{}

	// Default prefix
	t.Setenv("REDIS_KEY", "")
	key := s.redisKey("test-id-123")
	assert.Equal(t, "io.luzifer.ots:test-id-123", key)

	// Custom prefix
	t.Setenv("REDIS_KEY", "custom.prefix")
	key = s.redisKey("test-id-123")
	assert.Equal(t, "custom.prefix:test-id-123", key)
}

func TestRedisStorageFullLifecycle(t *testing.T) {
	srv, addr := startMockRedisServer(t)
	t.Cleanup(srv.Close)

	t.Setenv("REDIS_URL", strings.Replace(addr, "redis://", "tcp://", 1))

	store, err := New()
	require.NoError(t, err)
	require.NotNil(t, store)

	// Create a secret
	payload := []byte("confidential redis payload")
	id, err := store.Create(payload, time.Hour, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Count stored items
	count, err := store.Count()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Read and destroy
	retrieved, remaining, err := store.ReadAndDestroy(id)
	require.NoError(t, err)
	assert.Contains(t, string(retrieved), "confidential redis payload")
	assert.Equal(t, 0, remaining)

	// Reading destroyed secret returns ErrSecretNotFound
	_, _, err = store.ReadAndDestroy(id)
	assert.ErrorIs(t, err, storage.ErrSecretNotFound)

	// Create and Purge secret
	pId, err := store.Create([]byte("purge payload"), time.Hour, 1)
	require.NoError(t, err)

	purgedBytes, err := store.Purge(pId)
	require.NoError(t, err)
	assert.Contains(t, string(purgedBytes), "purge payload")

	// Purging missing secret returns ErrSecretNotFound
	_, err = store.Purge("missing-id")
	assert.ErrorIs(t, err, storage.ErrSecretNotFound)
}

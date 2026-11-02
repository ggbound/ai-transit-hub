package cache

import (
	"sync"
	"time"
)

// item 缓存条目
type item struct {
	value     interface{}
	expiresAt time.Time
}

// TTLCache 简单的进程内 TTL 缓存（并发安全）。
// 适用于低频变更的配置数据（provider/model、API key 列表、鉴权凭据），
// 缓存命中可省去每个请求的 DB 往返。
type TTLCache struct {
	mu    sync.RWMutex
	items map[string]item
	ttl   time.Duration
}

// New 创建 TTL 缓存，并启动一个与 TTL 同周期的过期清理协程，
// 避免长期运行时过期条目占用内存缓慢增长。
func New(ttl time.Duration) *TTLCache {
	c := &TTLCache{
		items: make(map[string]item),
		ttl:   ttl,
	}
	go c.janitor(ttl)
	return c
}

// Get 返回缓存值；key 不存在或已过期均视为未命中
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(it.expiresAt) {
		return nil, false
	}
	return it.value, true
}

// Set 写入缓存
func (c *TTLCache) Set(key string, value interface{}) {
	c.mu.Lock()
	c.items[key] = item{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// Delete 删除指定 key（配置变更时可主动失效）
func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// janitor 周期性清理过期条目
func (c *TTLCache) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, it := range c.items {
			if now.After(it.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

// 包级缓存实例：
var (
	// Config 低频配置数据缓存（provider/model、provider API key 列表），60s
	Config = New(60 * time.Second)

	// Auth 鉴权凭据缓存，30s。
	// 安全权衡：key 被禁用后最多在 30s TTL 窗口内仍可用，属可接受范围；
	// 过期时间（ExpiresAt）校验在缓存命中后仍会执行，不受缓存影响。
	Auth = New(30 * time.Second)
)

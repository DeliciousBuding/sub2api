package repository

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"

	"github.com/imroc/req/v3"
)

// reqClientOptions 定义 req 客户端的构建参数
type reqClientOptions struct {
	ProxyURL    string        // 代理 URL（支持 http/https/socks5）
	Timeout     time.Duration // 请求超时时间
	Impersonate bool          // 是否模拟 Chrome 浏览器指纹
	ForceHTTP2  bool          // 是否强制使用 HTTP/2
}

type sharedReqClientEntry struct {
	client    *req.Client
	expiresAt time.Time
	lastUsed  int64
}

type sharedReqClientPool struct {
	mu      sync.Mutex
	entries map[string]sharedReqClientEntry
}

var (
	// OAuth/隐私设置等辅助请求的 client 组合有限，不需要无界缓存。
	sharedReqClientTTL        = 15 * time.Minute
	sharedReqClientMaxEntries = 128
	sharedReqClients          = sharedReqClientPool{entries: make(map[string]sharedReqClientEntry)}
)

// getSharedReqClient 获取共享的 req 客户端实例
// 性能优化：相同配置复用同一客户端，避免重复创建
func getSharedReqClient(opts reqClientOptions) (*req.Client, error) {
	key := buildReqClientKey(opts)
	if cached, ok := sharedReqClients.get(key); ok {
		return cached, nil
	}

	client := req.C().SetTimeout(opts.Timeout)
	if opts.ForceHTTP2 {
		client = client.EnableForceHTTP2()
	}
	if opts.Impersonate {
		client = client.ImpersonateChrome()
	}
	trimmed, _, err := proxyurl.Parse(opts.ProxyURL)
	if err != nil {
		return nil, err
	}
	if trimmed != "" {
		client.SetProxyURL(trimmed)
	}

	return sharedReqClients.store(key, client), nil
}

func buildReqClientKey(opts reqClientOptions) string {
	return fmt.Sprintf("%s|%s|%t|%t",
		strings.TrimSpace(opts.ProxyURL),
		opts.Timeout.String(),
		opts.Impersonate,
		opts.ForceHTTP2,
	)
}

// CreatePrivacyReqClient creates an HTTP client for OpenAI privacy settings API
// This is exported for use by OpenAIPrivacyService
// Uses Chrome TLS fingerprint impersonation to bypass Cloudflare checks
func CreatePrivacyReqClient(proxyURL string) (*req.Client, error) {
	return getSharedReqClient(reqClientOptions{
		ProxyURL:    proxyURL,
		Timeout:     30 * time.Second,
		Impersonate: true, // Enable Chrome TLS fingerprint impersonation
	})
}

func (p *sharedReqClientPool) get(key string) (*req.Client, bool) {
	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleanupExpiredLocked(now)
	entry, ok := p.entries[key]
	if !ok || entry.client == nil {
		return nil, false
	}
	entry.expiresAt = now.Add(sharedReqClientTTL)
	entry.lastUsed = now.UnixNano()
	p.entries[key] = entry
	return entry.client, true
}

func (p *sharedReqClientPool) store(key string, client *req.Client) *req.Client {
	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.cleanupExpiredLocked(now)
	if entry, ok := p.entries[key]; ok && entry.client != nil && now.Before(entry.expiresAt) {
		entry.expiresAt = now.Add(sharedReqClientTTL)
		entry.lastUsed = now.UnixNano()
		p.entries[key] = entry
		closeReqClientIdle(client)
		return entry.client
	}

	p.entries[key] = sharedReqClientEntry{
		client:    client,
		expiresAt: now.Add(sharedReqClientTTL),
		lastUsed:  now.UnixNano(),
	}
	p.evictOverLimitLocked()
	return client
}

func (p *sharedReqClientPool) cleanupExpiredLocked(now time.Time) {
	for key, entry := range p.entries {
		if entry.client == nil || !now.Before(entry.expiresAt) {
			delete(p.entries, key)
			closeReqClientIdle(entry.client)
		}
	}
}

func (p *sharedReqClientPool) evictOverLimitLocked() {
	if sharedReqClientMaxEntries <= 0 {
		return
	}
	for len(p.entries) > sharedReqClientMaxEntries {
		var (
			oldestKey  string
			oldestTime int64
			found      bool
		)
		for key, entry := range p.entries {
			lastUsed := entry.lastUsed
			if !found || lastUsed < oldestTime {
				oldestKey = key
				oldestTime = lastUsed
				found = true
			}
		}
		if !found {
			return
		}
		entry := p.entries[oldestKey]
		delete(p.entries, oldestKey)
		closeReqClientIdle(entry.client)
	}
}

func closeReqClientIdle(client *req.Client) {
	if client == nil {
		return
	}
	transport := client.GetTransport()
	if transport != nil {
		transport.CloseIdleConnections()
	}
}

func (p *sharedReqClientPool) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, entry := range p.entries {
		delete(p.entries, key)
		closeReqClientIdle(entry.client)
	}
}

func (p *sharedReqClientPool) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

func (p *sharedReqClientPool) storeEntryForTest(key string, entry sharedReqClientEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.entries == nil {
		p.entries = make(map[string]sharedReqClientEntry)
	}
	p.entries[key] = entry
}

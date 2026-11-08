package middleware

import (
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

// 简单的基于令牌桶的限流实现，按 IP 维度缓存
type bucket struct {
    mu       sync.Mutex
    tokens   float64
    last     time.Time
    rate     float64 // 每秒补充的 token 数
    capacity int     // 桶容量
}

var (
    rateBuckets = make(map[string]*bucket)
    rlMu        sync.RWMutex
    sweepOnce   sync.Once
)

// bucketIdleTTL 是限流桶的空闲存活时间。桶表按限流键增长且永不回收会被
// 伪造 IP 耗尽内存，因此由清扫协程定期回收超过该时长未活跃的桶。
const bucketIdleTTL = 10 * time.Minute

// startBucketSweeper 定期回收空闲限流桶，防止桶表无限增长
func startBucketSweeper() {
    go func() {
        ticker := time.NewTicker(time.Minute)
        defer ticker.Stop()
        for now := range ticker.C {
            cutoff := now.Add(-bucketIdleTTL)
            rlMu.Lock()
            for key, b := range rateBuckets {
                b.mu.Lock()
                idle := b.last.Before(cutoff)
                b.mu.Unlock()
                if idle {
                    delete(rateBuckets, key)
                }
            }
            rlMu.Unlock()
        }
    }()
}

func getBucket(key string, capacity int) *bucket {
    rlMu.Lock()
    defer rlMu.Unlock()
    b, ok := rateBuckets[key]
    if !ok {
        b = &bucket{
            tokens:   float64(capacity),
            last:     time.Now(),
            rate:     float64(capacity) / 60.0, // per-second refill rate
            capacity: capacity,
        }
        rateBuckets[key] = b
    }
    return b
}

func (b *bucket) allow() bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    now := time.Now()
    elapsed := now.Sub(b.last).Seconds()
    b.last = now
    // 续期 token
    b.tokens += elapsed * b.rate
    if b.tokens > float64(b.capacity) {
        b.tokens = float64(b.capacity)
    }
    if b.tokens >= 1 {
        b.tokens -= 1
        return true
    }
    return false
}

// RateLimitMiddleware 基于 IP 的简单令牌桶限流
func RateLimitMiddleware() gin.HandlerFunc {
    sweepOnce.Do(startBucketSweeper)
    return func(c *gin.Context) {
        path := strings.ToLower(c.Request.URL.Path)
        var key string
        var capVal int
        if strings.Contains(path, "login") {
            // 登录接口限流：10 req/min
            key = c.ClientIP() + "|login"
            capVal = 10
        } else {
            // 默认限流：60 req/min
            key = c.ClientIP() + "|default"
            capVal = 60
        }
        b := getBucket(key, capVal)
        if b.allow() {
            c.Next()
            return
        }
        c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
    }
}

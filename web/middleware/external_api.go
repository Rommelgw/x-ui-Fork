package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// simple token bucket per ip
var rlStore = struct {
	sync.Mutex
	buckets map[string]*rateBucket
}{buckets: make(map[string]*rateBucket)}

type rateBucket struct {
	tokens     int
	lastRefill time.Time
}

func allow(ip string, ratePerMin int, burst int) bool {
	rlStore.Lock()
	defer rlStore.Unlock()
	b, ok := rlStore.buckets[ip]
	if !ok {
		b = &rateBucket{tokens: burst, lastRefill: time.Now()}
		rlStore.buckets[ip] = b
	}
	// refill
	elapsed := time.Since(b.lastRefill)
	if elapsed > 0 {
		added := int(elapsed.Minutes() * float64(ratePerMin))
		if added > 0 {
			b.tokens = min(burst, b.tokens+added)
			b.lastRefill = time.Now()
		}
	}
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func min(a, b int) int { if a < b { return a }; return b }

// ExternalAPIKeyMiddleware validates X-API-Key header against saved setting and rate-limits requests.
func ExternalAPIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		setting := service.SettingService{}
		expected, err := setting.GetExternalAPIKey()
		if err != nil || expected == "" || expected != apiKey {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		// rate limit per remote ip
		ip := clientIP(c.Request)
		if !allow(ip, 60, 30) { // 60 req/min, burst 30
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil { return h }
	return r.RemoteAddr
}

package easyframework

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	Mutex                sync.Mutex
	UserRequestsCount    *map[string]int
	MaxRequestsPerMinute int
}

func ShouldRequestBeRateLimited(context *Context, w http.ResponseWriter, r *http.Request) (requestCount int, shouldBeRateLimited bool) {
	rateLimiter := &context.RateLimiter

	rateLimiter.Mutex.Lock()
	defer rateLimiter.Mutex.Unlock()

	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	requests, ok := (*rateLimiter.UserRequestsCount)[host]
	requestCount = requests
	if !ok {
		(*rateLimiter.UserRequestsCount)[host] = 1
		return
	}
	if requests > rateLimiter.MaxRequestsPerMinute {
		shouldBeRateLimited = true
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	(*rateLimiter.UserRequestsCount)[host] += 1

	return
}

func RateLimiterRoutine(context *Context) {
	rateLimiter := &context.RateLimiter
	period := time.Minute
	for {
		time.Sleep(period)

		rateLimiter.Mutex.Lock()

		requestsPerUser := make(map[string]int)
		rateLimiter.UserRequestsCount = &requestsPerUser

		rateLimiter.Mutex.Unlock()
	}
}

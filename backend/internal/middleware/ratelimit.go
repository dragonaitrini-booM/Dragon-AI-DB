package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimit returns a middleware that limits requests per IP.
func RateLimit() func(http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = &client{limiter: rate.NewLimiter(2, 4)}
			}
			clients[ip].lastSeen = time.Now()
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

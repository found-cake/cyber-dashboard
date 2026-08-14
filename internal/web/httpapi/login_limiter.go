package httpapi

import (
	"sync"
	"time"
)

const (
	loginAttemptLimit  = 5
	loginAttemptWindow = time.Minute
	maxLoginClients    = 1024
)

type loginAttempts struct {
	count   int
	expires time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempts
	now      func() time.Time
}

func newLoginLimiter(now func() time.Time) *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempts), now: now}
}

func (l *loginLimiter) take(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key, value := range l.attempts {
		if !now.Before(value.expires) {
			delete(l.attempts, key)
		}
	}
	value, found := l.attempts[client]
	if !found {
		if len(l.attempts) >= maxLoginClients {
			return false
		}
		l.attempts[client] = loginAttempts{count: 1, expires: now.Add(loginAttemptWindow)}
		return true
	}
	if value.count >= loginAttemptLimit {
		return false
	}
	value.count++
	l.attempts[client] = value
	return true
}

func (l *loginLimiter) reset(client string) {
	l.mu.Lock()
	delete(l.attempts, client)
	l.mu.Unlock()
}

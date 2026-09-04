package mailer

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrRateLimitExceeded is returned when a profile has exceeded its hourly send limit
// and the caller should wait until the hour resets.
var ErrRateLimitExceeded = errors.New("rate limit exceeded for SMTP profile")

// ProfileUsage tracks the sending usage and limits for a single SMTP profile.
type ProfileUsage struct {
	SMTPID        int64
	MaxPerHour    int
	CurrentCount  int
	HourResetTime time.Time
	LastSentTime  time.Time
	MinDelay      time.Duration
}

// RateLimiter controls the send rate for multiple SMTP profiles.
// It enforces per-profile hourly send caps and minimum inter-send delays.
type RateLimiter struct {
	mu           sync.RWMutex
	profileUsage map[int64]*ProfileUsage
}

// NewRateLimiter returns an initialized RateLimiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		profileUsage: make(map[int64]*ProfileUsage),
	}
}

// RegisterProfile registers (or updates) the rate limits for a given SMTP profile.
func (rl *RateLimiter) RegisterProfile(smtpID int64, maxPerHour int, minDelay time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if existing, ok := rl.profileUsage[smtpID]; ok {
		existing.MaxPerHour = maxPerHour
		existing.MinDelay = minDelay
		return
	}

	rl.profileUsage[smtpID] = &ProfileUsage{
		SMTPID:        smtpID,
		MaxPerHour:    maxPerHour,
		CurrentCount:  0,
		HourResetTime: time.Now().Add(time.Hour),
		LastSentTime:  time.Time{},
		MinDelay:      minDelay,
	}
}

// WaitForSend blocks until a send is allowed for the given SMTP profile.
// It enforces both the hourly cap and the minimum delay between sends.
// Returns ErrRateLimitExceeded if the hourly cap has been reached and the
// hour has not yet reset.
func (rl *RateLimiter) WaitForSend(smtpID int64) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	usage, ok := rl.profileUsage[smtpID]
	if !ok {
		// Unregistered profiles are allowed without restriction.
		return nil
	}

	now := time.Now()

	// If the hourly window has expired, reset the counter.
	if now.After(usage.HourResetTime) {
		usage.CurrentCount = 0
		usage.HourResetTime = now.Add(time.Hour)
	}

	// Check if the hourly cap is reached.
	if usage.CurrentCount >= usage.MaxPerHour {
		return &RateLimitError{
			SMTPID:    smtpID,
			ResetTime: usage.HourResetTime,
		}
	}

	// Enforce the minimum delay between sends.
	if usage.MinDelay > 0 && !usage.LastSentTime.IsZero() {
		elapsed := now.Sub(usage.LastSentTime)
		if elapsed < usage.MinDelay {
			waitTime := usage.MinDelay - elapsed
			// Release the lock while sleeping to allow other goroutines to proceed.
			rl.mu.Unlock()
			time.Sleep(waitTime)
			rl.mu.Lock()

			// Re-check the hourly cap after sleeping in case the window reset
			// or another goroutine consumed the quota.
			now = time.Now()
			if now.After(usage.HourResetTime) {
				usage.CurrentCount = 0
				usage.HourResetTime = now.Add(time.Hour)
			}
			if usage.CurrentCount >= usage.MaxPerHour {
				return &RateLimitError{
					SMTPID:    smtpID,
					ResetTime: usage.HourResetTime,
				}
			}
		}
	}

	usage.CurrentCount++
	usage.LastSentTime = time.Now()
	return nil
}

// GetUsage returns a copy of the current usage for the given SMTP profile.
// Returns nil if the profile is not registered.
func (rl *RateLimiter) GetUsage(smtpID int64) *ProfileUsage {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	usage, ok := rl.profileUsage[smtpID]
	if !ok {
		return nil
	}

	// Return a copy to avoid external mutation.
	copy := *usage
	return &copy
}

// RateLimitError provides details about why a rate limit was hit.
type RateLimitError struct {
	SMTPID    int64
	ResetTime time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for SMTP profile %d, resets at %s",
		e.SMTPID, e.ResetTime.Format(time.RFC3339))
}

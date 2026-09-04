package mailer

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
)

// SelectionStrategy determines how an SMTP profile is chosen from a pool.
type SelectionStrategy int

const (
	// RoundRobin cycles through SMTP profiles in order.
	RoundRobin SelectionStrategy = iota
	// Random picks an SMTP profile at random.
	Random
	// LeastUsed picks the SMTP profile with the lowest current usage count.
	LeastUsed
)

// BatchMail represents a group of emails to be sent through a specific SMTP profile.
type BatchMail struct {
	Mails       []Mail
	SMTPID      int64
	CampaignID  int64
}

// MultiSMTPWorker processes BatchMail instances from a queue, respecting
// per-SMTP rate limits and selection strategies.
type MultiSMTPWorker struct {
	queue        chan *BatchMail
	rateLimiter  *RateLimiter
	strategy     SelectionStrategy
	mu           sync.Mutex
	roundRobinIdx int
	smtpPool     []int64
}

// NewMultiSMTPWorker returns an initialized MultiSMTPWorker.
func NewMultiSMTPWorker(rl *RateLimiter, strategy SelectionStrategy, queueSize int) *MultiSMTPWorker {
	if queueSize <= 0 {
		queueSize = 100
	}
	return &MultiSMTPWorker{
		queue:       make(chan *BatchMail, queueSize),
		rateLimiter: rl,
		strategy:    strategy,
		smtpPool:    []int64{},
	}
}

// SetSMTPPool sets the pool of available SMTP profile IDs for selection strategies.
func (w *MultiSMTPWorker) SetSMTPPool(smtpIDs []int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.smtpPool = make([]int64, len(smtpIDs))
	copy(w.smtpPool, smtpIDs)
}

// Start launches the worker to begin listening on the queue channel
// for new BatchMail instances to process.
func (w *MultiSMTPWorker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-w.queue:
			go func(ctx context.Context, batch *BatchMail) {
				err := w.sendMailWithDelay(ctx, batch)
				if err != nil {
					log.WithFields(map[string]interface{}{
						"smtp_id":     batch.SMTPID,
						"campaign_id": batch.CampaignID,
						"error":       err.Error(),
					}).Warn("Failed to send batch")
				}
			}(ctx, batch)
		}
	}
}

// Queue adds a BatchMail to the internal processing queue.
func (w *MultiSMTPWorker) Queue(batch *BatchMail) {
	w.queue <- batch
}

// SelectSMTP returns the next SMTP profile ID based on the configured strategy.
func (w *MultiSMTPWorker) SelectSMTP() (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.smtpPool) == 0 {
		return 0, errors.New("no SMTP profiles available in pool")
	}

	switch w.strategy {
	case RoundRobin:
		idx := w.roundRobinIdx % len(w.smtpPool)
		w.roundRobinIdx++
		return w.smtpPool[idx], nil

	case Random:
		return w.smtpPool[rand.Intn(len(w.smtpPool))], nil

	case LeastUsed:
		var bestID int64
		bestCount := int(^uint(0) >> 1) // MaxInt
		for _, id := range w.smtpPool {
			usage := w.rateLimiter.GetUsage(id)
			if usage == nil {
				return id, nil
			}
			if usage.CurrentCount < bestCount {
				bestCount = usage.CurrentCount
				bestID = id
			}
		}
		return bestID, nil

	default:
		return w.smtpPool[0], nil
	}
}

// sendMailWithDelay sends all mails in a batch, respecting the rate limiter's
// per-profile constraints. It waits for rate limit clearance before sending
// each individual mail.
func (w *MultiSMTPWorker) sendMailWithDelay(ctx context.Context, batch *BatchMail) error {
	if len(batch.Mails) == 0 {
		return nil
	}

	dialer, err := batch.Mails[0].GetDialer()
	if err != nil {
		errorMail(err, batch.Mails)
		return err
	}

	for _, m := range batch.Mails {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Wait for rate limiter clearance before sending.
		err := w.rateLimiter.WaitForSend(batch.SMTPID)
		if err != nil {
			if errors.Is(err, ErrRateLimitExceeded) {
				m.Backoff(err)
				continue
			}
			m.Error(err)
			continue
		}

		sendMail(ctx, dialer, []Mail{m})
	}

	return nil
}

// QueueWithSelection creates a BatchMail from a slice of Mail and the campaign ID,
// selects an SMTP profile using the configured strategy, and queues it.
func (w *MultiSMTPWorker) QueueWithSelection(ms []Mail, campaignID int64) error {
	smtpID, err := w.SelectSMTP()
	if err != nil {
		return err
	}

	batch := &BatchMail{
		Mails:      ms,
		SMTPID:     smtpID,
		CampaignID: campaignID,
	}
	w.Queue(batch)
	return nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

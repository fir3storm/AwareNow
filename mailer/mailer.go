package mailer

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/textproto"
	"sync"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/gophish/gomail"
	"github.com/sirupsen/logrus"
)

// MaxReconnectAttempts is the maximum number of times we should reconnect to a server
var MaxReconnectAttempts = 10

// ErrMaxConnectAttempts is thrown when the maximum number of reconnect attempts
// is reached.
type ErrMaxConnectAttempts struct {
	underlyingError error
}

// Error returns the wrapped error response
func (e *ErrMaxConnectAttempts) Error() string {
	errString := "Max connection attempts exceeded"
	if e.underlyingError != nil {
		errString = fmt.Sprintf("%s - %s", errString, e.underlyingError.Error())
	}
	return errString
}

// Mailer is an interface that defines an object used to queue and
// send mailer.Mail instances.
type Mailer interface {
	Start(ctx context.Context)
	Queue([]Mail)
}

// Sender exposes the common operations required for sending email.
type Sender interface {
	Send(from string, to []string, msg io.WriterTo) error
	Close() error
	Reset() error
}

// Dialer dials to an SMTP server and returns the SendCloser
type Dialer interface {
	Dial() (Sender, error)
}

// Mail is an interface that handles the common operations for email messages
type Mail interface {
	Backoff(reason error) error
	Error(err error) error
	Success() error
	Generate(msg *gomail.Message) error
	GetDialer() (Dialer, error)
	GetSmtpFrom() (string, error)
}

// MailWorker is the worker that receives slices of emails
// on a channel to send. It's assumed that every slice of emails received is meant
// to be sent to the same server.
type MailWorker struct {
	queue       chan []Mail
	batchQueue  chan *BatchMail
	rateLimiter *RateLimiter
	sendDelay   time.Duration
	wg          sync.WaitGroup
}

// NewMailWorker returns an instance of MailWorker with the mail queue
// initialized.
func NewMailWorker() *MailWorker {
	return &MailWorker{
		queue:       make(chan []Mail),
		batchQueue:  make(chan *BatchMail, 100),
		rateLimiter: NewRateLimiter(),
		sendDelay:   0,
	}
}

// NewMailWorkerWithRateLimiter returns an instance of MailWorker with a
// custom rate limiter and optional delay between individual emails.
func NewMailWorkerWithRateLimiter(rl *RateLimiter, sendDelay time.Duration) *MailWorker {
	return &MailWorker{
		queue:       make(chan []Mail),
		batchQueue:  make(chan *BatchMail, 100),
		rateLimiter: rl,
		sendDelay:   sendDelay,
	}
}

// Start launches the mail worker to begin listening on the Queue channel
// for new slices of Mail instances to process.
func (mw *MailWorker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			mw.wg.Wait()
			return
		case ms := <-mw.queue:
			mw.wg.Add(1)
			go func(ctx context.Context, ms []Mail) {
				defer mw.wg.Done()
				dialer, err := ms[0].GetDialer()
				if err != nil {
					errorMail(err, ms)
					return
				}
				sendMailWithDelay(ctx, dialer, ms, mw.sendDelay)
			}(ctx, ms)
		case batch := <-mw.batchQueue:
			mw.wg.Add(1)
			go func(ctx context.Context, batch *BatchMail) {
				defer mw.wg.Done()
				dialer, err := batch.Mails[0].GetDialer()
				if err != nil {
					errorMail(err, batch.Mails)
					return
				}
				sendBatchMail(ctx, dialer, batch, mw.rateLimiter)
			}(ctx, batch)
		}
	}
}

// Queue sends the provided mail to the internal queue for processing.
func (mw *MailWorker) Queue(ms []Mail) {
	mw.queue <- ms
}

// QueueBatch sends a BatchMail to the internal batch queue for processing
// with rate limiting support.
func (mw *MailWorker) QueueBatch(batch *BatchMail) {
	mw.batchQueue <- batch
}

// GetRateLimiter returns the MailWorker's rate limiter for profile registration.
func (mw *MailWorker) GetRateLimiter() *RateLimiter {
	return mw.rateLimiter
}

// errorMail is a helper to handle erroring out a slice of Mail instances
// in the case that an unrecoverable error occurs.
func errorMail(err error, ms []Mail) {
	for _, m := range ms {
		m.Error(err)
	}
}

// dialHost attempts to make a connection to the host specified by the Dialer.
// It returns MaxReconnectAttempts if the number of connection attempts has been
// exceeded.
func dialHost(ctx context.Context, dialer Dialer) (Sender, error) {
	sendAttempt := 0
	var sender Sender
	var err error
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			break
		}
		sender, err = dialer.Dial()
		if err == nil {
			break
		}
		sendAttempt++
		if sendAttempt == MaxReconnectAttempts {
			err = &ErrMaxConnectAttempts{
				underlyingError: err,
			}
			break
		}
		// Exponential backoff: 2^attempt seconds (capped at 60s)
		backoffDuration := time.Duration(math.Pow(2, float64(sendAttempt))) * time.Second
		if backoffDuration > 60*time.Second {
			backoffDuration = 60 * time.Second
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
		}
	}
	return sender, err
}

// sendMail attempts to send the provided Mail instances.
// If the context is cancelled before all of the mail are sent,
// sendMail just returns and does not modify those emails.
func sendMail(ctx context.Context, dialer Dialer, ms []Mail) {
	sendMailWithDelay(ctx, dialer, ms, 0)
}

// sendMailWithDelay attempts to send the provided Mail instances with an optional
// delay between each email. If the context is cancelled before all of the mail
// are sent, sendMailWithDelay just returns and does not modify those emails.
func sendMailWithDelay(ctx context.Context, dialer Dialer, ms []Mail, delay time.Duration) {
	sender, err := dialHost(ctx, dialer)
	if err != nil {
		log.Warn(err)
		errorMail(err, ms)
		return
	}
	defer sender.Close()
	message := gomail.NewMessage()
	for i, m := range ms {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}

		// Apply delay between emails (but not before the first one).
		if delay > 0 && i > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		message.Reset()
		err = m.Generate(message)
		if err != nil {
			log.Warn(err)
			m.Error(err)
			continue
		}

		smtp_from, err := m.GetSmtpFrom()
		if err != nil {
			m.Error(err)
			continue
		}

		err = gomail.SendCustomFrom(sender, smtp_from, message)
		if err != nil {
			if te, ok := err.(*textproto.Error); ok {
				switch {
				// If it's a temporary error, we should backoff and try again later.
				// We'll reset the connection so future messages don't incur a
				// different error (see https://github.com/fir3storm/AwareNow/issues/787).
				case te.Code >= 400 && te.Code <= 499:
					log.WithFields(logrus.Fields{
						"code":  te.Code,
						"email": message.GetHeader("To")[0],
					}).Warn(err)
					m.Backoff(err)
					sender.Reset()
					continue
				// Otherwise, if it's a permanent error, we shouldn't backoff this message,
				// since the RFC specifies that running the same commands won't work next time.
				// We should reset our sender and error this message out.
				case te.Code >= 500 && te.Code <= 599:
					log.WithFields(logrus.Fields{
						"code":  te.Code,
						"email": message.GetHeader("To")[0],
					}).Warn(err)
					m.Error(err)
					sender.Reset()
					continue
				// If something else happened, let's just error out and reset the
				// sender
				default:
					log.WithFields(logrus.Fields{
						"code":  "unknown",
						"email": message.GetHeader("To")[0],
					}).Warn(err)
					m.Error(err)
					sender.Reset()
					continue
				}
			} else {
				// This likely indicates that something happened to the underlying
				// connection. We'll try to reconnect and, if that fails, we'll
				// error out the remaining emails.
				log.WithFields(logrus.Fields{
					"email": message.GetHeader("To")[0],
				}).Warn(err)
				origErr := err
				sender, err = dialHost(ctx, dialer)
				if err != nil {
					errorMail(err, ms[i:])
					break
				}
				m.Backoff(origErr)
				continue
			}
		}
		log.WithFields(logrus.Fields{
			"smtp_from":     smtp_from,
			"envelope_from": message.GetHeader("From")[0],
			"email":         message.GetHeader("To")[0],
		}).Info("Email sent")
		m.Success()
	}
}

// sendBatchMail sends a BatchMail using the provided rate limiter to control
// per-SMTP-profile send rates. Each email in the batch is gated by the rate
// limiter's WaitForSend call.
func sendBatchMail(ctx context.Context, dialer Dialer, batch *BatchMail, rl *RateLimiter) {
	sender, err := dialHost(ctx, dialer)
	if err != nil {
		log.Warn(err)
		errorMail(err, batch.Mails)
		return
	}
	defer sender.Close()
	message := gomail.NewMessage()
	for i, m := range batch.Mails {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}

		// Wait for rate limiter clearance before sending.
		err := rl.WaitForSend(batch.SMTPID)
		if err != nil {
			log.WithFields(logrus.Fields{
				"smtp_id":     batch.SMTPID,
				"campaign_id": batch.CampaignID,
			}).Warn(err)
			m.Backoff(err)
			continue
		}

		message.Reset()
		err = m.Generate(message)
		if err != nil {
			log.Warn(err)
			m.Error(err)
			continue
		}

		smtp_from, err := m.GetSmtpFrom()
		if err != nil {
			m.Error(err)
			continue
		}

		err = gomail.SendCustomFrom(sender, smtp_from, message)
		if err != nil {
			if te, ok := err.(*textproto.Error); ok {
				switch {
				case te.Code >= 400 && te.Code <= 499:
					log.WithFields(logrus.Fields{
						"code":  te.Code,
						"email": message.GetHeader("To")[0],
					}).Warn(err)
					m.Backoff(err)
					sender.Reset()
					continue
				case te.Code >= 500 && te.Code <= 599:
					log.WithFields(logrus.Fields{
						"code":  te.Code,
						"email": message.GetHeader("To")[0],
					}).Warn(err)
					m.Error(err)
					sender.Reset()
					continue
				default:
					log.WithFields(logrus.Fields{
						"code":  "unknown",
						"email": message.GetHeader("To")[0],
					}).Warn(err)
					m.Error(err)
					sender.Reset()
					continue
				}
			} else {
				log.WithFields(logrus.Fields{
					"email": message.GetHeader("To")[0],
				}).Warn(err)
				origErr := err
				sender, err = dialHost(ctx, dialer)
				if err != nil {
					errorMail(err, batch.Mails[i:])
					break
				}
				m.Backoff(origErr)
				continue
			}
		}
		log.WithFields(logrus.Fields{
			"smtp_id":       batch.SMTPID,
			"campaign_id":   batch.CampaignID,
			"smtp_from":     smtp_from,
			"envelope_from": message.GetHeader("From")[0],
			"email":         message.GetHeader("To")[0],
		}).Info("Batch email sent")
		m.Success()
	}
}

package worker

import (
	"context"
	"math/rand"
	"sync"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/mailer"
	"github.com/fir3storm/AwareNow/models"
	"github.com/sirupsen/logrus"
)

// Worker is an interface that defines the operations needed for a background worker
type Worker interface {
	Start()
	LaunchCampaign(c models.Campaign)
	SendTestEmail(s *models.EmailRequest) error
}

// DefaultWorker is the background worker that handles watching for new campaigns and sending emails appropriately.
type DefaultWorker struct {
	mailer mailer.Mailer
}

// New creates a new worker object to handle the creation of campaigns
func New(options ...func(Worker) error) (Worker, error) {
	defaultMailer := mailer.NewMailWorker()
	w := &DefaultWorker{
		mailer: defaultMailer,
	}
	for _, opt := range options {
		if err := opt(w); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// WithMailer sets the mailer for a given worker.
// By default, workers use a standard, default mailworker.
func WithMailer(m mailer.Mailer) func(*DefaultWorker) error {
	return func(w *DefaultWorker) error {
		w.mailer = m
		return nil
	}
}

// smtpSendState tracks the current state for round-robin SMTP selection
type smtpSendState struct {
	mu             sync.Mutex
	currentIndex   int
	smtpSentCounts map[int64]int64 // smtp_id -> count sent in this batch
}

// newSMTPState creates a new SMTP send state tracker
func newSMTPState() *smtpSendState {
	return &smtpSendState{
		smtpSentCounts: make(map[int64]int64),
	}
}

// selectSMTPIndex selects an SMTP index based on the campaign's selection strategy
func selectSMTPIndex(state *smtpSendState, campaignSMTPs []models.CampaignSMTP, strategy string, maxPerProfile int64) int {
	state.mu.Lock()
	defer state.mu.Unlock()

	if len(campaignSMTPs) == 0 {
		return -1
	}

	switch strategy {
	case models.SelectionStrategyRandom:
		return selectRandomSMTP(state, campaignSMTPs, maxPerProfile)
	case models.SelectionStrategyLeastUsed:
		return selectLeastUsedSMTP(state, campaignSMTPs, maxPerProfile)
	default: // round_robin or any other value defaults to round_robin
		return selectRoundRobinSMTP(state, campaignSMTPs, maxPerProfile)
	}
}

// selectRoundRobinSMTP selects the next SMTP in round-robin order
func selectRoundRobinSMTP(state *smtpSendState, campaignSMTPs []models.CampaignSMTP, maxPerProfile int64) int {
	n := len(campaignSMTPs)
	for i := 0; i < n; i++ {
		idx := state.currentIndex % n
		state.currentIndex++
		smtpID := campaignSMTPs[idx].SMTPId
		if maxPerProfile <= 0 || state.smtpSentCounts[smtpID] < maxPerProfile {
			return idx
		}
	}
	// All SMTPs at max, fall back to first
	return 0
}

// selectRandomSMTP selects a random SMTP that hasn't exceeded its limit
func selectRandomSMTP(state *smtpSendState, campaignSMTPs []models.CampaignSMTP, maxPerProfile int64) int {
	// Build list of available SMTP indices
	available := make([]int, 0, len(campaignSMTPs))
	for i, cs := range campaignSMTPs {
		if maxPerProfile <= 0 || state.smtpSentCounts[cs.SMTPId] < maxPerProfile {
			available = append(available, i)
		}
	}
	if len(available) == 0 {
		return 0 // All at max, fall back to first
	}
	return available[rand.Intn(len(available))]
}

// selectLeastUsedSMTP selects the SMTP with the lowest sent count
func selectLeastUsedSMTP(state *smtpSendState, campaignSMTPs []models.CampaignSMTP, maxPerProfile int64) int {
	bestIdx := 0
	bestCount := int64(-1)
	for i, cs := range campaignSMTPs {
		count := state.smtpSentCounts[cs.SMTPId]
		if maxPerProfile > 0 && count >= maxPerProfile {
			continue
		}
		if bestCount == -1 || count < bestCount {
			bestCount = count
			bestIdx = i
		}
	}
	return bestIdx
}

// distributeEmails distributes mail entries across SMTP profiles based on the selection strategy.
// Returns a map of smtpID -> []Mail for grouped sending.
func distributeEmails(ms []mailer.Mail, campaignSMTPs []models.CampaignSMTP, dc models.DeliveryConfig) map[int64][]mailer.Mail {
	if len(campaignSMTPs) == 0 {
		// No multi-SMTP config, return all under key 0 (legacy behavior)
		return map[int64][]mailer.Mail{0: ms}
	}

	state := newSMTPState()
	grouped := make(map[int64][]mailer.Mail)

	for _, m := range ms {
		idx := selectSMTPIndex(state, campaignSMTPs, dc.SelectionStrategy, dc.MaxEmailsPerProfile)
		if idx < 0 {
			idx = 0
		}
		smtpID := campaignSMTPs[idx].SMTPId
		grouped[smtpID] = append(grouped[smtpID], m)
		state.mu.Lock()
		state.smtpSentCounts[smtpID]++
		state.mu.Unlock()
	}

	return grouped
}

// processCampaigns loads maillogs scheduled to be sent before the provided
// time and sends them to the mailer.
func (w *DefaultWorker) processCampaigns(t time.Time) error {
	ms, err := models.GetQueuedMailLogs(t.UTC())
	if err != nil {
		log.Error(err)
		return err
	}
	// Lock the MailLogs (they will be unlocked after processing)
	err = models.LockMailLogs(ms, true)
	if err != nil {
		return err
	}
	campaignCache := make(map[int64]models.Campaign)
	// We'll group the maillogs by campaign ID to (roughly) group
	// them by sending profile. This lets the mailer re-use the Sender
	// instead of having to re-connect to the SMTP server for every
	// email.
	msg := make(map[int64][]mailer.Mail)
	for _, m := range ms {
		// We cache the campaign here to greatly reduce the time it takes to
		// generate the message (ref #1726)
		c, ok := campaignCache[m.CampaignId]
		if !ok {
			c, err = models.GetCampaignMailContext(m.CampaignId, m.UserId)
			if err != nil {
				return err
			}
			campaignCache[c.Id] = c
		}
		m.CacheCampaign(&c)
		msg[m.CampaignId] = append(msg[m.CampaignId], m)
	}

	// Next, we process each group of maillogs in parallel
	for cid, msc := range msg {
		go func(cid int64, msc []mailer.Mail) {
			c := campaignCache[cid]
			if c.Status == models.CampaignQueued {
				err := c.UpdateStatus(models.CampaignInProgress)
				if err != nil {
					log.Error(err)
					return
				}
			}

			// Load multi-SMTP relationships for this campaign
			campaignSMTPs, err := models.GetCampaignSMTPs(cid)
			if err != nil {
				log.Warnf("Error loading campaign SMTPs for campaign %d: %v", cid, err)
			}

			// Build delivery config from campaign
			dc := c.DeliveryConfig
			if dc.SelectionStrategy == "" {
				dc.SelectionStrategy = models.DefaultSelectionStrategy
			}

			// If we have multiple SMTPs, distribute emails across them
			if len(campaignSMTPs) > 1 {
				log.WithFields(logrus.Fields{
					"campaign_id":      cid,
					"num_smtps":        len(campaignSMTPs),
					"strategy":         dc.SelectionStrategy,
					"delay_between_ms": dc.DelayBetweenMs,
				}).Info("Distributing emails across multiple SMTP profiles")

				grouped := distributeEmails(msc, campaignSMTPs, dc)

				// Send each SMTP group, optionally with delay
				for smtpID, group := range grouped {
					// Check rate limits for this SMTP
					smtpConfig := getSMTPConfig(campaignSMTPs, smtpID)
					if smtpConfig != nil && smtpConfig.MaxEmailsPerHour > 0 {
						if !models.CanSendFromSMTP(smtpID, smtpConfig.MaxEmailsPerHour) {
							log.WithFields(logrus.Fields{
								"smtp_id": smtpID,
								"limit":   smtpConfig.MaxEmailsPerHour,
							}).Warn("SMTP rate limit reached, skipping this SMTP for now")
							continue
						}
					}

					log.WithFields(logrus.Fields{
						"campaign_id": cid,
						"smtp_id":     smtpID,
						"num_emails":  len(group),
					}).Info("Sending email group to mailer")
					w.mailer.Queue(group)

					// Increment usage tracking
					for i := int64(0); i < int64(len(group)); i++ {
						models.IncrementSMTPUsage(smtpID)
					}

					// Apply delay between SMTP groups if configured
					if dc.DelayBetweenMs > 0 {
						time.Sleep(time.Duration(dc.DelayBetweenMs) * time.Millisecond)
					}
				}
			} else {
				// Single SMTP or legacy mode - send all at once
				log.WithFields(logrus.Fields{
					"num_emails": len(msc),
				}).Info("Sending emails to mailer for processing")
				w.mailer.Queue(msc)

				// Increment usage for the single SMTP
				if len(campaignSMTPs) == 1 {
					for i := int64(0); i < int64(len(msc)); i++ {
						models.IncrementSMTPUsage(campaignSMTPs[0].SMTPId)
					}
				}
			}
		}(cid, msc)
	}
	return nil
}

// getSMTPConfig finds the SMTP config from campaignSMTPs by SMTP ID
func getSMTPConfig(campaignSMTPs []models.CampaignSMTP, smtpID int64) *models.SMTP {
	for i := range campaignSMTPs {
		if campaignSMTPs[i].SMTPId == smtpID {
			return &campaignSMTPs[i].SMTP
		}
	}
	return nil
}

// Start launches the worker to poll the database every minute for any pending maillogs
// that need to be processed.
func (w *DefaultWorker) Start() {
	log.Info("Background Worker Started Successfully - Waiting for Campaigns")
	go w.mailer.Start(context.Background())
	for t := range time.Tick(1 * time.Minute) {
		err := w.processCampaigns(t)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

// LaunchCampaign starts a campaign
func (w *DefaultWorker) LaunchCampaign(c models.Campaign) {
	ms, err := models.GetMailLogsByCampaign(c.Id)
	if err != nil {
		log.Error(err)
		return
	}
	models.LockMailLogs(ms, true)
	// This is required since you cannot pass a slice of values
	// that implements an interface as a slice of that interface.
	mailEntries := []mailer.Mail{}
	currentTime := time.Now().UTC()
	campaignMailCtx, err := models.GetCampaignMailContext(c.Id, c.UserId)
	if err != nil {
		log.Error(err)
		return
	}

	// Load multi-SMTP relationships
	campaignSMTPs, smtpErr := models.GetCampaignSMTPs(c.Id)
	if smtpErr != nil {
		log.Warnf("Error loading campaign SMTPs: %v", smtpErr)
	}

	// Build delivery config
	dc := c.DeliveryConfig
	if dc.SelectionStrategy == "" {
		dc.SelectionStrategy = models.DefaultSelectionStrategy
	}

	for _, m := range ms {
		// Only send the emails scheduled to be sent for the past minute to
		// respect the campaign scheduling options
		if m.SendDate.After(currentTime) {
			m.Unlock()
			continue
		}
		err = m.CacheCampaign(&campaignMailCtx)
		if err != nil {
			log.Error(err)
			return
		}
		mailEntries = append(mailEntries, m)
	}

	// If we have multiple SMTPs, distribute emails
	if len(campaignSMTPs) > 1 {
		log.WithFields(logrus.Fields{
			"campaign_id": c.Id,
			"num_smtps":   len(campaignSMTPs),
			"strategy":    dc.SelectionStrategy,
		}).Info("Launching multi-SMTP campaign")

		grouped := distributeEmails(mailEntries, campaignSMTPs, dc)
		for smtpID, group := range grouped {
			log.WithFields(logrus.Fields{
				"campaign_id": c.Id,
				"smtp_id":     smtpID,
				"num_emails":  len(group),
			}).Info("Queuing email group for campaign launch")
			w.mailer.Queue(group)

			// Increment usage tracking
			for i := int64(0); i < int64(len(group)); i++ {
				models.IncrementSMTPUsage(smtpID)
			}

			// Apply delay between SMTP groups if configured
			if dc.DelayBetweenMs > 0 {
				time.Sleep(time.Duration(dc.DelayBetweenMs) * time.Millisecond)
			}
		}
	} else {
		// Legacy single-SMTP mode
		w.mailer.Queue(mailEntries)

		// Increment usage for the single SMTP
		if len(campaignSMTPs) == 1 {
			for i := int64(0); i < int64(len(mailEntries)); i++ {
				models.IncrementSMTPUsage(campaignSMTPs[0].SMTPId)
			}
		}
	}
}

// SendTestEmail sends a test email
func (w *DefaultWorker) SendTestEmail(s *models.EmailRequest) error {
	go func() {
		ms := []mailer.Mail{s}
		w.mailer.Queue(ms)
	}()
	return <-s.ErrorChan
}

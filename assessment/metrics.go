// Package assessment implements the USP-1 measurement specification
// (docs/superpowers/specs/2026-09-06-usp-measurement-spec.md): Recognition,
// Discrimination, Recovery, and Speed metrics for the "Proof of Resilience"
// assessment concept, plus their uncertainty intervals.
//
// This package is deliberately independent of the GORM-backed models
// package and of any persisted Assessment/Scenario/Cohort schema (neither
// exists yet — that's USP-2's job). It operates on plain in-memory
// Recipient records so its arithmetic can be validated against
// hand-checked fixtures before any schema or API is built on top of it,
// per the plan's own requirement. Future USP-2/3/4 work should translate
// persisted campaign/result/event rows into Recipient values and call into
// this package, not reimplement this arithmetic elsewhere.
package assessment

import (
	"math"
	"sort"
	"time"
)

// ScenarioKind distinguishes a simulated-threat scenario from a benign
// control message. Recognition/Recovery/Speed apply only to threat
// scenarios; Discrimination applies only to benign ones (spec §3).
type ScenarioKind int

const (
	ScenarioThreat ScenarioKind = iota
	ScenarioBenign
)

// EventKind is one observed state-transition kind. Sent is not represented
// here: eligibility (spec §3.1) is a property of Recipient.Sent, not an
// event to be ordered against the others.
type EventKind int

const (
	EventOpened EventKind = iota
	EventClicked
	EventDataSubmit
	EventReported
)

// severity gives the canonical tie-break order from spec §2:
// Opened < Clicked < DataSubmit < Reported.
func (k EventKind) severity() int {
	switch k {
	case EventOpened:
		return 1
	case EventClicked:
		return 2
	case EventDataSubmit:
		return 3
	case EventReported:
		return 4
	default:
		return 0
	}
}

// Event is a single observed state-transition for a recipient. Offset is
// measured from that recipient's own SendDate, per spec §1: the
// observation window starts at each recipient's individual send time, not
// a single wall-clock cutoff shared by the whole cohort.
type Event struct {
	Kind   EventKind
	Offset time.Duration
}

// Recipient is one assigned recipient's full observation record for a
// single assessment phase.
type Recipient struct {
	// Sent is false when the send failed before dispatch. Such recipients
	// are excluded from every metric's denominator entirely (spec §3.1) —
	// a delivery failure is not a behavioral observation.
	Sent     bool
	Scenario ScenarioKind
	Events   []Event
}

// sortedEvents returns r's events ordered by Offset ascending, breaking
// exact-Offset ties by canonical severity (spec §2).
func (r Recipient) sortedEvents() []Event {
	events := append([]Event(nil), r.Events...)
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Offset != events[j].Offset {
			return events[i].Offset < events[j].Offset
		}
		return events[i].Kind.severity() < events[j].Kind.severity()
	})
	return events
}

// firstOffset returns the earliest in-window Offset among events of the
// given kinds, deduplicating repeated events of the same kind per spec §2
// ("use only the first occurrence of each event type").
func (r Recipient) firstOffset(window time.Duration, kinds ...EventKind) (time.Duration, bool) {
	best := time.Duration(0)
	found := false
	for _, e := range r.sortedEvents() {
		if e.Offset > window {
			continue
		}
		for _, k := range kinds {
			if e.Kind != k {
				continue
			}
			if !found || e.Offset < best {
				best = e.Offset
				found = true
			}
		}
	}
	return best, found
}

// firstRiskyInteraction returns the earliest in-window Clicked or
// DataSubmit event (spec §2's "first risky interaction").
func (r Recipient) firstRiskyInteraction(window time.Duration) (time.Duration, bool) {
	return r.firstOffset(window, EventClicked, EventDataSubmit)
}

// firstReport returns the earliest in-window Reported event.
func (r Recipient) firstReport(window time.Duration) (time.Duration, bool) {
	return r.firstOffset(window, EventReported)
}

// MinCohortSize is the spec §3.5 default small-cohort suppression
// threshold: a metric computed over fewer than this many eligible
// recipients is displayed as "insufficient data" rather than as a
// percentage. This is a starting default, not empirically validated, and
// is named here specifically so it can be revisited from one place.
const MinCohortSize = 10

// Proportion is a computed rate with its 95% Wilson score confidence
// interval (spec §3.5) and small-cohort suppression flag.
type Proportion struct {
	Numerator   int
	Denominator int
	Rate        float64
	CILow       float64
	CIHigh      float64
	// Suppressed is true when Denominator < MinCohortSize. Rate/CILow/CIHigh
	// are still computed (so tests and internal tooling can inspect them)
	// but callers presenting this to a user must show "insufficient data
	// (n=<Denominator>)" instead of Rate when Suppressed is true.
	Suppressed bool
}

// wilsonInterval computes the 95% Wilson score interval for a proportion of
// numerator successes out of denominator trials, per spec §3.5's formula
// (z = 1.959964 for 95%).
func wilsonInterval(numerator, denominator int) (low, high float64) {
	if denominator == 0 {
		return 0, 0
	}
	n := float64(denominator)
	p := float64(numerator) / n
	const z = 1.959964
	center := (p + z*z/(2*n)) / (1 + z*z/n)
	half := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / (1 + z*z/n)
	return center - half, center + half
}

func newProportion(numerator, denominator int) Proportion {
	p := Proportion{Numerator: numerator, Denominator: denominator}
	if denominator == 0 {
		return p
	}
	p.Rate = float64(numerator) / float64(denominator)
	p.CILow, p.CIHigh = wilsonInterval(numerator, denominator)
	p.Suppressed = denominator < MinCohortSize
	return p
}

// eligibleThreat reports whether r counts toward a threat-scenario metric's
// denominator: it was actually sent and belongs to a threat scenario.
func eligibleThreat(r Recipient) bool {
	return r.Sent && r.Scenario == ScenarioThreat
}

// Recognition computes the spec §3.1 metric: the share of eligible
// simulated-threat recipients who reported before any risky interaction
// (or who never had one) within window.
func Recognition(recipients []Recipient, window time.Duration) Proportion {
	eligible, recognized := 0, 0
	for _, r := range recipients {
		if !eligibleThreat(r) {
			continue
		}
		eligible++
		risk, hasRisk := r.firstRiskyInteraction(window)
		report, hasReport := r.firstReport(window)
		if hasReport && (!hasRisk || report < risk) {
			recognized++
		}
	}
	return newProportion(recognized, eligible)
}

// Discrimination computes the spec §3.2 metric: the share of eligible
// benign-control recipients who reported the message as if it were a
// threat within window.
func Discrimination(recipients []Recipient, window time.Duration) Proportion {
	eligible, reported := 0, 0
	for _, r := range recipients {
		if !r.Sent || r.Scenario != ScenarioBenign {
			continue
		}
		eligible++
		if _, has := r.firstReport(window); has {
			reported++
		}
	}
	return newProportion(reported, eligible)
}

// Recovery computes the spec §3.3 metric: among eligible recipients who did
// have a risky interaction, the share who nonetheless reported afterward,
// within window.
func Recovery(recipients []Recipient, window time.Duration) Proportion {
	riskySubset, recovered := 0, 0
	for _, r := range recipients {
		if !eligibleThreat(r) {
			continue
		}
		risk, hasRisk := r.firstRiskyInteraction(window)
		if !hasRisk {
			continue
		}
		riskySubset++
		report, hasReport := r.firstReport(window)
		if hasReport && report >= risk {
			recovered++
		}
	}
	return newProportion(recovered, riskySubset)
}

// SpeedResult is the spec §3.4 time-to-report distribution plus the
// nonreporting proportion, computed over the same eligible set as
// Recognition.
type SpeedResult struct {
	Eligible         int
	AnyReportCount   int
	Nonreporting     Proportion
	Median, P25, P75 time.Duration
}

// percentile computes p (in [0,1]) over a sorted slice using linear
// interpolation between ranks, per spec §3.4's documented, deterministic
// method. sorted must be non-empty and already ascending.
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 1 {
		return sorted[0]
	}
	rank := p * float64(n-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	span := sorted[hi] - sorted[lo]
	return sorted[lo] + time.Duration(frac*float64(span))
}

// Speed computes the spec §3.4 metric over the threat-scenario eligible
// set (spec: "same eligible set as Recognition").
func Speed(recipients []Recipient, window time.Duration) SpeedResult {
	eligible := 0
	var reportTimes []time.Duration
	for _, r := range recipients {
		if !eligibleThreat(r) {
			continue
		}
		eligible++
		if t, has := r.firstReport(window); has {
			reportTimes = append(reportTimes, t)
		}
	}
	sort.Slice(reportTimes, func(i, j int) bool { return reportTimes[i] < reportTimes[j] })

	result := SpeedResult{
		Eligible:       eligible,
		AnyReportCount: len(reportTimes),
		Nonreporting:   newProportion(eligible-len(reportTimes), eligible),
	}
	if len(reportTimes) > 0 {
		result.Median = percentile(reportTimes, 0.5)
		result.P25 = percentile(reportTimes, 0.25)
		result.P75 = percentile(reportTimes, 0.75)
	}
	return result
}

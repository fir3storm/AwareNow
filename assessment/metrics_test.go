package assessment

import (
	"math"
	"testing"
	"time"
)

// fixtureRecipients builds the exact 12-recipient worked fixture from
// docs/superpowers/specs/2026-09-06-usp-measurement-spec.md §4. Every
// expected value in this test file was independently verified with a
// standalone script before being written into the spec, and must be kept
// in sync with it — if you change this fixture, update the spec's table
// and recomputed figures too, not just this file.
func fixtureRecipients() []Recipient {
	hours := func(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }
	threat := func(events ...Event) Recipient {
		return Recipient{Sent: true, Scenario: ScenarioThreat, Events: events}
	}
	return []Recipient{
		threat(Event{EventReported, hours(1)}),                                  // r1
		threat(Event{EventReported, hours(4)}),                                  // r2
		threat(Event{EventClicked, hours(2)}, Event{EventReported, hours(10)}),  // r3
		threat(Event{EventClicked, hours(0.5)}),                                 // r4
		threat(Event{EventDataSubmit, hours(1)}),                                // r5
		threat(Event{EventOpened, hours(3)}),                                    // r6
		threat(),                                                                // r7
		threat(Event{EventReported, hours(0.25)}),                               // r8
		threat(Event{EventClicked, hours(20)}, Event{EventReported, hours(30)}), // r9
		threat(Event{EventReported, hours(50)}),                                 // r10
		threat(Event{EventClicked, hours(71)}),                                  // r11
		threat(Event{EventReported, hours(80)}),                                 // r12: outside the 72h window
	}
}

const fixtureWindow = 72 * time.Hour

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

func TestRecognitionFixture(t *testing.T) {
	got := Recognition(fixtureRecipients(), fixtureWindow)
	if got.Denominator != 12 {
		t.Fatalf("eligible = %d, want 12", got.Denominator)
	}
	if got.Numerator != 4 {
		t.Fatalf("recognized = %d, want 4 (r1, r2, r8, r10)", got.Numerator)
	}
	if !almostEqual(got.Rate, 4.0/12.0) {
		t.Fatalf("rate = %.4f, want %.4f", got.Rate, 4.0/12.0)
	}
	if got.Suppressed {
		t.Fatal("n=12 should clear the MinCohortSize=10 suppression threshold")
	}
	if !almostEqual(got.CILow, 0.1381) || !almostEqual(got.CIHigh, 0.6094) {
		t.Fatalf("Wilson interval = (%.4f, %.4f), want (0.1381, 0.6094)", got.CILow, got.CIHigh)
	}
}

func TestRecoveryFixture(t *testing.T) {
	got := Recovery(fixtureRecipients(), fixtureWindow)
	if got.Denominator != 5 {
		t.Fatalf("risky subset = %d, want 5 (r3, r4, r5, r9, r11)", got.Denominator)
	}
	if got.Numerator != 2 {
		t.Fatalf("recovered = %d, want 2 (r3, r9)", got.Numerator)
	}
	if !almostEqual(got.Rate, 0.4) {
		t.Fatalf("rate = %.4f, want 0.4000", got.Rate)
	}
}

func TestSpeedFixture(t *testing.T) {
	got := Speed(fixtureRecipients(), fixtureWindow)
	if got.Eligible != 12 {
		t.Fatalf("eligible = %d, want 12", got.Eligible)
	}
	if got.AnyReportCount != 6 {
		t.Fatalf("any-report count = %d, want 6 (r1, r2, r3, r8, r9, r10)", got.AnyReportCount)
	}
	if got.Nonreporting.Numerator != 6 || got.Nonreporting.Denominator != 12 {
		t.Fatalf("nonreporting = %d/%d, want 6/12", got.Nonreporting.Numerator, got.Nonreporting.Denominator)
	}
	if !almostEqual(got.Nonreporting.Rate, 0.5) {
		t.Fatalf("nonreporting rate = %.4f, want 0.5000", got.Nonreporting.Rate)
	}
	wantMedian := 7 * time.Hour
	wantP25 := time.Duration(1.75 * float64(time.Hour))
	wantP75 := 25 * time.Hour
	if got.Median != wantMedian {
		t.Errorf("median = %v, want %v", got.Median, wantMedian)
	}
	if got.P25 != wantP25 {
		t.Errorf("p25 = %v, want %v", got.P25, wantP25)
	}
	if got.P75 != wantP75 {
		t.Errorf("p75 = %v, want %v", got.P75, wantP75)
	}
}

func TestDiscriminationNoThreatContamination(t *testing.T) {
	// A benign cohort is independent of the threat fixture above: a benign
	// recipient who reports is discrimination, never recognition, and a
	// threat-scenario recipient must never be counted toward
	// Discrimination's denominator even if fields happen to overlap.
	recipients := []Recipient{
		{Sent: true, Scenario: ScenarioBenign, Events: []Event{{EventReported, time.Hour}}},
		{Sent: true, Scenario: ScenarioBenign, Events: nil},
		{Sent: true, Scenario: ScenarioThreat, Events: []Event{{EventReported, time.Hour}}},
	}
	got := Discrimination(recipients, fixtureWindow)
	if got.Denominator != 2 {
		t.Fatalf("eligible = %d, want 2 (only the benign recipients)", got.Denominator)
	}
	if got.Numerator != 1 {
		t.Fatalf("reported-as-threat = %d, want 1", got.Numerator)
	}
}

func TestUnsentRecipientsExcludedEntirely(t *testing.T) {
	recipients := []Recipient{
		{Sent: false, Scenario: ScenarioThreat, Events: []Event{{EventReported, time.Hour}}},
	}
	if got := Recognition(recipients, fixtureWindow); got.Denominator != 0 {
		t.Fatalf("Recognition denominator = %d, want 0 for an unsent recipient", got.Denominator)
	}
	if got := Recovery(recipients, fixtureWindow); got.Denominator != 0 {
		t.Fatalf("Recovery denominator = %d, want 0 for an unsent recipient", got.Denominator)
	}
	if got := Speed(recipients, fixtureWindow); got.Eligible != 0 {
		t.Fatalf("Speed eligible = %d, want 0 for an unsent recipient", got.Eligible)
	}
}

func TestSmallCohortSuppression(t *testing.T) {
	recipients := make([]Recipient, MinCohortSize-1)
	for i := range recipients {
		recipients[i] = Recipient{Sent: true, Scenario: ScenarioThreat}
	}
	got := Recognition(recipients, fixtureWindow)
	if !got.Suppressed {
		t.Fatalf("n=%d should be suppressed (threshold %d)", got.Denominator, MinCohortSize)
	}
}

func TestRepeatedEventsDoNotDoubleCount(t *testing.T) {
	// Two Clicked events for the same recipient must still count as one
	// risky interaction at its earliest offset, per spec §2.
	r := Recipient{Sent: true, Scenario: ScenarioThreat, Events: []Event{
		{EventClicked, 5 * time.Hour},
		{EventClicked, 1 * time.Hour},
		{EventReported, 2 * time.Hour},
	}}
	risk, has := r.firstRiskyInteraction(fixtureWindow)
	if !has || risk != time.Hour {
		t.Fatalf("firstRiskyInteraction = %v, %v; want 1h, true", risk, has)
	}
	// Reported at 2h is after the earliest click at 1h, so this recipient
	// is Recovered, not Recognized.
	recognized := Recognition([]Recipient{r}, fixtureWindow)
	if recognized.Numerator != 0 {
		t.Fatalf("recognized = %d, want 0 (reported after the earliest risky interaction)", recognized.Numerator)
	}
	recovered := Recovery([]Recipient{r}, fixtureWindow)
	if recovered.Numerator != 1 {
		t.Fatalf("recovered = %d, want 1", recovered.Numerator)
	}
}

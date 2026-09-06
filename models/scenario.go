package models

import (
	"errors"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// ScenarioKindThreat marks a scenario as a simulated-phishing message used
// to measure recognition/recovery (see assessment.ScenarioThreat).
const ScenarioKindThreat = "threat"

// ScenarioKindBenign marks a scenario as a legitimate-looking control
// message that should NOT be reported as phishing (see
// assessment.ScenarioBenign).
const ScenarioKindBenign = "benign"

// ScenarioStatusDraft indicates a scenario awaiting reviewer approval.
const ScenarioStatusDraft = "draft"

// ScenarioStatusApproved indicates a reviewer has signed off on a
// scenario's sanitized content and it may be used in an assessment.
const ScenarioStatusApproved = "approved"

var (
	// ErrScenarioNotFound indicates no scenario was found for the given criteria.
	ErrScenarioNotFound = errors.New("scenario not found")
	// ErrScenarioOwnerRequired prevents creation without a trusted owner,
	// mirroring the same control added to ReportedMessage during the P0 review.
	ErrScenarioOwnerRequired = errors.New("scenario owner required")
	// ErrScenarioInvalidKind rejects a kind other than threat/benign.
	ErrScenarioInvalidKind = errors.New("scenario kind must be \"threat\" or \"benign\"")
	// ErrScenarioUnsafeContent is returned by ValidateScenarioSafety (and by
	// CreateScenario, which always calls it) when scenario HTML still
	// contains a live external destination or an active/scriptable element.
	ErrScenarioUnsafeContent = errors.New("scenario content is not sanitized")
)

// Scenario is a sanitized, versioned message used as the stimulus in an
// Assessment. It always has a provenance link back to the real, reviewed
// ReportedMessage it was derived from — this repo intentionally does not
// support authoring a scenario from scratch, so every scenario traces to
// a real incident an admin already reviewed.
//
// "Sanitized" is enforced mechanically only for what this package can
// actually verify: no live external links, scripts, or active resource
// loads (see ValidateScenarioSafety). It is NOT a claim that the content
// contains no secrets or sensitive information in some broader sense —
// that judgment call belongs to the human reviewer who must still approve
// the scenario (ReviewedBy/ReviewedAt) before it can be used.
type Scenario struct {
	ID                      int64     `json:"id" gorm:"column:id; primary_key:yes"`
	OwnerID                 int64     `json:"-" gorm:"column:owner_id; index"`
	SourceReportedMessageID int64     `json:"source_reported_message_id" gorm:"column:source_reported_message_id; index"`
	Name                    string    `json:"name" gorm:"column:name; not null"`
	SkillTag                string    `json:"skill_tag" gorm:"column:skill_tag; not null"`
	Kind                    string    `json:"kind" gorm:"column:kind; not null"`
	Version                 int64     `json:"version" gorm:"column:version; not null"`
	Subject                 string    `json:"subject" gorm:"column:subject"`
	HTML                    string    `json:"html" gorm:"column:html; sql:type:text"`
	Text                    string    `json:"text" gorm:"column:text; sql:type:text"`
	Status                  string    `json:"status" gorm:"column:status; not null"`
	ReviewedBy              string    `json:"reviewed_by" gorm:"column:reviewed_by"`
	ReviewedAt              time.Time `json:"reviewed_at" gorm:"column:reviewed_at"`
	CreatedAt               time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName specifies the table name for the Scenario model.
func (Scenario) TableName() string {
	return "scenarios"
}

// ValidateScenarioSafety rejects HTML that still contains a live external
// link, an active/scriptable element, or a resource load pointing anywhere
// other than this application's own template variable. It does not, and
// cannot, verify the absence of sensitive information in the visible text
// — that remains the reviewer's judgment call, enforced by the separate
// ApproveScenario step.
//
// Rules (deliberately conservative — reject rather than guess at intent).
// Revised 2026-09-07 after an internal review found three bypasses in the
// first version (a <base> tag retargeting an in-page "#..." anchor to an
// external origin, an external image load via <img srcset> instead of
// src, and a CSS url()/@import load via <style> or a style="..."
// attribute) — none of those are hypothetical, all three were verified as
// live loads/navigations before being closed here:
//   - <script>, <iframe>, <object>, <embed>, <link>, <base>, <style>,
//     <meta>, <video>, <audio>, <source>, <track> are never allowed.
//   - <a href>, <img src>, <form action> must be exactly "{{.URL}}" (this
//     application's existing tracking-link placeholder, the same one
//     ParseEmailContent already rewrites real links to), a same-page
//     anchor ("#..."), a "data:" URI (self-contained, no network request),
//     or empty. Anything else — an absolute http(s) URL, a bare domain, a
//     "mailto:", "javascript:", etc. — is rejected by name so a reviewer
//     can see exactly what needs to be fixed.
//   - <img srcset>/<source srcset> candidate URLs are checked the same way
//     as src — src alone is not sufficient, srcset carries its own list.
//   - Any style="..." attribute containing "url(" or "@import" is rejected
//     (the <style> element covers the same vector at the whole-document
//     level; this covers it per-element).
//   - Inline "on*" event handler attributes (onclick, onerror, ...) are
//     rejected outright regardless of which element they're on.
//
// This is still not an exhaustive HTML-sanitization guarantee — it is a
// deny-list against the concrete vectors identified so far, not a formally
// verified allow-list. Treat a clean result as "no known bypass found",
// not as an unconditional safety proof.
func ValidateScenarioSafety(html string) error {
	if strings.TrimSpace(html) == "" {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return err
	}

	var offense string
	check := func(msg string) {
		if offense == "" {
			offense = msg
		}
	}

	// <base> is blocked outright rather than treated as a destination to
	// validate: if present, it silently changes what a same-page "#..."
	// anchor actually resolves to, which would otherwise let a same-page
	// link masquerade as safe while navigating to an external base URL.
	// <style>/<meta>/<video>/<audio>/<source>/<track> are blocked outright
	// too: a <style> block (or any style="..." attribute, checked below)
	// can load an external resource via CSS url()/@import, a <meta
	// http-equiv="refresh"> can force navigation, and video/audio elements
	// carry their own live src/poster surface this function does not
	// otherwise inspect.
	doc.Find("script, iframe, object, embed, link, base, style, meta, video, audio, source, track").Each(func(_ int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		check("disallowed <" + tag + "> element")
	})

	isSafeDestination := func(v string) bool {
		v = strings.TrimSpace(v)
		if v == "" || v == "{{.URL}}" || strings.HasPrefix(v, "#") {
			return true
		}
		return strings.HasPrefix(strings.ToLower(v), "data:")
	}

	checkAttr := func(sel string, attr string) {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			v, exists := s.Attr(attr)
			if !exists {
				return
			}
			if !isSafeDestination(v) {
				check("disallowed " + attr + " destination: " + v)
			}
		})
	}
	checkAttr("a", "href")
	checkAttr("img", "src")
	checkAttr("form", "action")

	// srcset carries its own candidate URLs independent of src (e.g.
	// `srcset="https://evil.example/track.png 1x"`) and is not covered by
	// the plain src check above.
	doc.Find("img[srcset], source[srcset]").Each(func(_ int, s *goquery.Selection) {
		v, _ := s.Attr("srcset")
		for _, candidate := range strings.Split(v, ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			// Each candidate is "<url> <descriptor>?"; the URL is the part
			// before the first run of whitespace.
			url := strings.Fields(candidate)[0]
			if !isSafeDestination(url) {
				check("disallowed srcset destination: " + url)
			}
		}
	})

	// A url()/@import inside an inline style="..." attribute is the same
	// external-resource-load vector as a <style> block, just scoped to one
	// element instead of blocking the whole tag.
	doc.Find("[style]").Each(func(_ int, s *goquery.Selection) {
		v, _ := s.Attr("style")
		lower := strings.ToLower(v)
		if strings.Contains(lower, "url(") || strings.Contains(lower, "@import") {
			check("disallowed external reference in style attribute")
		}
	})

	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		for _, node := range s.Nodes {
			for _, a := range node.Attr {
				if strings.HasPrefix(strings.ToLower(a.Key), "on") {
					check("disallowed inline event handler: " + a.Key)
				}
			}
		}
	})

	if offense != "" {
		return errorsWrap(ErrScenarioUnsafeContent, offense)
	}
	return nil
}

// errorsWrap is a tiny local helper so callers can still compare against
// ErrScenarioUnsafeContent with errors.Is while getting a specific reason
// in the error text, without pulling in fmt.Errorf's %w formatting rules
// at every call site above.
func errorsWrap(sentinel error, detail string) error {
	return &scenarioSafetyError{sentinel: sentinel, detail: detail}
}

type scenarioSafetyError struct {
	sentinel error
	detail   string
}

func (e *scenarioSafetyError) Error() string { return e.sentinel.Error() + ": " + e.detail }
func (e *scenarioSafetyError) Unwrap() error { return e.sentinel }

// CreateScenario saves a new scenario after validating ownership, kind, and
// content safety. New scenarios always start in ScenarioStatusDraft — only
// ApproveScenario may move one to ScenarioStatusApproved.
func CreateScenario(s *Scenario) error {
	if s.OwnerID <= 0 {
		return ErrScenarioOwnerRequired
	}
	if _, err := GetUser(s.OwnerID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrScenarioOwnerRequired
		}
		return err
	}
	if s.Kind != ScenarioKindThreat && s.Kind != ScenarioKindBenign {
		return ErrScenarioInvalidKind
	}
	if err := ValidateScenarioSafety(s.HTML); err != nil {
		return err
	}
	if s.Version <= 0 {
		s.Version = 1
	}
	s.Status = ScenarioStatusDraft
	s.CreatedAt = time.Now().UTC()
	err := db.Create(s).Error
	if err != nil {
		log.Errorf("error creating scenario: %v", err)
	}
	return err
}

// GetScenarios returns every scenario owned by ownerID, most recent first.
func GetScenarios(ownerID int64) ([]Scenario, error) {
	scenarios := []Scenario{}
	err := db.Where("owner_id = ? AND owner_id > 0", ownerID).Order("created_at desc").Find(&scenarios).Error
	if err != nil {
		log.Errorf("error getting scenarios: %v", err)
	}
	return scenarios, err
}

// GetScenarioByID retrieves a single scenario by ID, scoped to ownerID.
func GetScenarioByID(id, ownerID int64) (Scenario, error) {
	s := Scenario{}
	err := db.Where("id = ? AND owner_id = ? AND owner_id > 0", id, ownerID).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return s, ErrScenarioNotFound
		}
		log.Errorf("error getting scenario by id: %v", err)
	}
	return s, err
}

// ApproveScenario records a reviewer's sign-off. This is the human
// judgment step ValidateScenarioSafety cannot substitute for — it exists
// specifically so a scenario cannot be used in an assessment on the
// strength of the mechanical safety check alone.
func ApproveScenario(id, ownerID int64, reviewedBy string) error {
	result := db.Model(&Scenario{}).
		Where("id = ? AND owner_id = ? AND owner_id > 0", id, ownerID).
		Updates(map[string]interface{}{
			"status":      ScenarioStatusApproved,
			"reviewed_by": reviewedBy,
			"reviewed_at": time.Now().UTC(),
		})
	if result.Error != nil {
		log.Errorf("error approving scenario: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrScenarioNotFound
	}
	return nil
}

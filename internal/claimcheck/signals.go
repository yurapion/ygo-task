package claimcheck

import (
	"strings"
	"time"
)

// signal is a phrase that asserts a field value when it appears in a feed.
//
// Matching is substring and case-insensitive, which is deliberately crude: it
// is here to surface a contradiction the feed already contains, not to
// understand the text. It will miss a policy stated in German, and it will miss
// a negated phrase ("no kids club"). Both are recorded as gaps rather than
// hidden behind a parser that looks cleverer than it is.
type signal struct {
	phrase string
	field  string
	value  string
}

// proseSignals read a free-text description. This is where the binding policy
// usually lives, and where a structured schema never looks.
var proseSignals = []signal{
	{"adults-only", "policy.children", "false"},
	{"adults only", "policy.children", "false"},
	{"no children", "policy.children", "false"},
}

// facilitySignals read a structured amenity or feature list. A facility for
// children is an assertion that children are allowed, which is why it can
// contradict the prose above.
var facilitySignals = []signal{
	{"kids club", "policy.children", "true"},
	{"kids' club", "policy.children", "true"},
	{"children's club", "policy.children", "true"},
	{"childrens club", "policy.children", "true"},
}

// policyClaims are what a record's prose and its structured fields each assert
// about policy, attributed to the field they came from rather than to the feed.
//
// Keying on the field path is what makes this case detectable at all: on this
// data both claims come from one scrape of one hotel, so a source key of
// "scrape-booking-sites" would see a single uncontested voice.
func policyClaims(rec Record) []Claim {
	c := claimSet{observed: observedAt(rec)}

	description := strings.ToLower(rec.Description)
	for _, s := range proseSignals {
		if strings.Contains(description, s.phrase) {
			c.add(s.field, s.value, rec.Source+"/description")
		}
	}

	for _, feature := range rec.Features {
		lowered := strings.ToLower(feature)
		for _, s := range facilitySignals {
			if strings.Contains(lowered, s.phrase) {
				c.add(s.field, s.value, rec.Source+"/features")
			}
		}
	}

	return c.claims
}

// claimSet collects claims while dropping exact repeats, so two phrasings of
// the same signal in one field do not look like two independent sources.
type claimSet struct {
	observed time.Time
	seen     map[string]bool
	claims   []Claim
}

func (c *claimSet) add(field, value, source string) {
	key := field + "\x00" + value + "\x00" + source
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.claims = append(c.claims, Claim{
		Field: field, Value: value, Source: source, ObservedAt: c.observed,
	})
}

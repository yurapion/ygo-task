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
	// blockedBy are phrases that contain this one while meaning the opposite.
	// A bare substring test cannot see negation: "Kein Restaurant" contains
	// "restaurant", so without this the same six words assert both that the
	// hotel has a restaurant and that it does not, and the report invents a
	// conflict inside a single unambiguous sentence.
	blockedBy []string
}

// fires reports whether the signal's phrase is present and not negated.
func (s signal) fires(text string) bool {
	if !strings.Contains(text, s.phrase) {
		return false
	}
	for _, b := range s.blockedBy {
		if strings.Contains(text, b) {
			return false
		}
	}
	return true
}

// proseSignals read a free-text description. This is where the binding policy
// usually lives, and where a structured schema never looks.
var proseSignals = []signal{
	{phrase: "adults-only", field: "policy.children", value: "false"},
	{phrase: "adults only", field: "policy.children", value: "false"},
	{phrase: "no children", field: "policy.children", value: "false"},

	// A hotel's negative facts live almost entirely in prose, and in whatever
	// language the feed was written in. "Kein Restaurant" is the only denial in
	// this dataset, and the record's structured amenities field is null — so
	// the schema view loses both this and the breakfast beside it.
	{phrase: "kein restaurant", field: "amenity.restaurant", value: "false"},
	{phrase: "no restaurant", field: "amenity.restaurant", value: "false"},
	{phrase: "restaurant", field: "amenity.restaurant", value: "true",
		blockedBy: []string{"kein restaurant", "no restaurant", "without restaurant", "ohne restaurant"}},
	{phrase: "frühstück inklusive", field: "amenity.breakfast", value: "true"},
	{phrase: "fruhstuck inklusive", field: "amenity.breakfast", value: "true"},
	{phrase: "breakfast included", field: "amenity.breakfast", value: "true"},
	{phrase: "includes breakfast", field: "amenity.breakfast", value: "true"},
}

// facilitySignals read a structured amenity or feature list. A facility for
// children is an assertion that children are allowed, which is why it can
// contradict the prose above.
var facilitySignals = []signal{
	{phrase: "kids club", field: "policy.children", value: "true"},
	{phrase: "kids' club", field: "policy.children", value: "true"},
	{phrase: "children's club", field: "policy.children", value: "true"},
	{phrase: "childrens club", field: "policy.children", value: "true"},
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
		if s.fires(description) {
			c.add(s.field, s.value, rec.Source+"/description")
		}
	}

	for _, feature := range rec.Features {
		lowered := strings.ToLower(feature)
		for _, s := range facilitySignals {
			if s.fires(lowered) {
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

// Package claimcheck groups hotel records from mixed feeds into properties and
// reports what each feed claims about them, without resolving disagreements.
package claimcheck

import (
	"encoding/json"
	"io"
	"time"
)

// Status is what the set of claims for one field looks like taken together.
type Status string

const (
	// Supported means at least one source asserts a value and all agree. The
	// value may be "false": an affirmative denial is support, not absence.
	Supported Status = "supported"
	// Conflict means two or more sources assert incompatible values.
	Conflict Status = "conflict"
	// Absent means no source asserts anything about the field.
	Absent Status = "absent"
)

// Staleness is independent of Status: a field is routinely both in conflict and
// out of date, and collapsing the two into one enum loses whichever is reported
// second.
type Staleness string

const (
	Fresh   Staleness = "fresh"
	Stale   Staleness = "stale"
	Unknown Staleness = "unknown"
)

// FreshWindow is how old an observation may be before it is reported stale.
const FreshWindow = 365 * 24 * time.Hour

// Claim is one source asserting one value for one field.
//
// Source is a feed name joined to the field path it came from, not the feed
// alone, because the sharpest errors in this data are a single record
// disagreeing with itself.
type Claim struct {
	Field      string
	Value      string
	Source     string
	ObservedAt time.Time // zero value means the feed carried no timestamp
}

// Finding is every claim about one field of one property, plus what they add up
// to.
type Finding struct {
	Field     string
	Status    Status
	Staleness Staleness
	Claims    []Claim
}

// Unparsed is something a feed published that this tool did not understand.
//
// It exists so that the limits of the vocabulary are visible in the output
// rather than inferred from its absence. A facility we cannot canonicalise, a
// description no signal matched, and a JSON key with no struct field behind it
// are all cases where silence and comprehension look identical, and silence is
// the more dangerous of the two.
type Unparsed struct {
	Kind   string // "field", "facility" or "description"
	Name   string // the JSON key, for Kind "field"; empty otherwise
	Text   string // the raw text, or the raw JSON value for Kind "field"
	Source string
}

// Property is a real-world hotel, which may be described by several records.
type Property struct {
	Key      string
	Name     string
	Sources  []string
	Findings []Finding
	Unparsed []Unparsed
}

// Coords is a decimal lat/lng pair as it appears in the partner feeds.
type Coords struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Record is the union of every field shape seen across the feeds. Pointer and
// slice fields distinguish "the feed said null or nothing" from "the feed said
// an empty value", which is a distinction the report depends on.
type Record struct {
	Source         string   `json:"source"`
	HotelName      string   `json:"hotel_name"`
	Name           string   `json:"name"`
	City           string   `json:"city"`
	Country        string   `json:"country"`
	Location       string   `json:"location"`
	Stars          *int     `json:"stars"`
	Rating         string   `json:"rating"`
	Description    string   `json:"description"`
	Amenities      *string  `json:"amenities"`
	Features       []string `json:"features"`
	ReviewSnippets []string `json:"review_snippets"`
	Coords         *Coords  `json:"coords"`
	PriceFromEUR   *int     `json:"price_from_eur"`
	PriceFrom      string   `json:"price_from"`
	LastSeen       string   `json:"last_seen"`

	// UnknownFields holds every JSON key this struct has no home for, mapped to
	// its raw value. Populated by UnmarshalJSON; never decoded from the feed.
	UnknownFields map[string]string `json:"-"`
}

// LoadRecords decodes a feed dump.
func LoadRecords(r io.Reader) ([]Record, error) {
	var recs []Record
	if err := json.NewDecoder(r).Decode(&recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// FindingFor returns the finding for a field, and whether one was reported.
func (p Property) FindingFor(field string) (Finding, bool) {
	for _, f := range p.Findings {
		if f.Field == field {
			return f, true
		}
	}
	return Finding{}, false
}

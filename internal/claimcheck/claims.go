package claimcheck

import (
	"cmp"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"
)

// unresolvedCoords counts coordinate pairs the lookup could not place. It is a
// process counter, not a per-run one: the rate is the thing worth alerting on.
var unresolvedCoords atomic.Int64

// UnresolvedCoords is how many records carried coordinates that matched no
// known city, and so went without a location cross-check.
func UnresolvedCoords() int64 { return unresolvedCoords.Load() }

// knownFields is the set of fields the report always speaks about. A field is
// only ever "absent" relative to a list like this one — without it, a field
// nobody claimed is indistinguishable from a field nobody thought to ask about.
var knownFields = append([]string{"city", "stars", "policy.children"}, amenityFields()...)

// lastSeenLayout is the date format the scrape feed uses.
const lastSeenLayout = "2006-01-02"

// Check groups records and reports the claim status of every known field.
func Check(recs []Record, now time.Time) []Property {
	groups := groupRecords(recs)
	props := make([]Property, 0, len(groups))
	for _, g := range groups {
		p := g.property()
		claims, unparsed := extract(g.records)
		p.Findings = findings(claims, now)
		sortUnparsed(unparsed)
		countUnparsed(unparsed)
		p.Unparsed = unparsed
		props = append(props, p)
	}
	return props
}

// extract pulls every claim each record makes, and everything it published that
// this tool could not place.
func extract(recs []Record) ([]Claim, []Unparsed) {
	var claims []Claim
	var unparsed []Unparsed
	for _, rec := range recs {
		claims = append(claims, cityClaims(rec)...)
		claims = append(claims, starClaims(rec)...)

		policy, proseUnparsed := policyClaims(rec)
		claims = append(claims, policy...)
		unparsed = append(unparsed, proseUnparsed...)

		amenities, facilityUnparsed := amenityClaims(rec)
		claims = append(claims, amenities...)
		unparsed = append(unparsed, facilityUnparsed...)

		unparsed = append(unparsed, unknownFieldUnparsed(rec)...)
	}
	return claims, unparsed
}

// cityClaims are what a record says, and what its coordinates imply, about
// which city the property is in.
//
// The coordinate claim is attributed to "derived:coords" rather than to the
// feed, so that a record contradicting itself produces two comparable claims.
// Attributing both to the feed would make the contradiction invisible: one
// source, no disagreement, reported as supported.
func cityClaims(rec Record) []Claim {
	observed := observedAt(rec)
	var out []Claim

	if rec.City != "" {
		out = append(out, Claim{
			Field: "city", Value: rec.City,
			Source: rec.Source + "/city", ObservedAt: observed,
		})
	}
	if city := locationCity(rec); city != "" {
		out = append(out, Claim{
			Field: "city", Value: city,
			Source: rec.Source + "/location", ObservedAt: observed,
		})
	}
	if rec.Coords != nil {
		// A coordinate outside the lookup asserts nothing. Naming the nearest
		// entry regardless of distance would manufacture a conflict.
		city, ok := nearestCity(*rec.Coords)
		if !ok {
			// This is the report's one silent degradation: a property whose
			// location is never cross-checked looks the same as one that was
			// checked and agreed. Counting it is how a lookup that has fallen
			// behind the feed becomes visible instead of just quiet.
			slog.Warn("coordinates matched no known city; location left unchecked",
				"source", rec.Source, "name", recordName(rec),
				"lat", rec.Coords.Lat, "lng", rec.Coords.Lng)
			unresolvedCoords.Add(1)
		} else {
			out = append(out, Claim{
				Field: "city", Value: city,
				Source: "derived:coords", ObservedAt: observed,
			})
		}
	}
	return out
}

// locationCity is the city half of a joined "City, Country" location string.
func locationCity(rec Record) string {
	if rec.City != "" || rec.Location == "" {
		return ""
	}
	return statedCity(rec)
}

// observedAt is when the feed says it saw this, or the zero time when it does
// not say. Zero is "unknown", never "now": treating an undated record as fresh
// is how three-year-old data gets presented as current.
func observedAt(rec Record) time.Time {
	if rec.LastSeen == "" {
		return time.Time{}
	}
	t, err := time.Parse(lastSeenLayout, rec.LastSeen)
	if err != nil {
		// An unparseable date is not a reason to drop the record's claims, and
		// it is not a reason to call them fresh. It is unknown, like no date.
		return time.Time{}
	}
	return t
}

// findings reports every known field, including the ones nobody claimed.
func findings(claims []Claim, now time.Time) []Finding {
	byField := map[string][]Claim{}
	for _, c := range claims {
		byField[c.Field] = append(byField[c.Field], c)
	}

	out := make([]Finding, 0, len(knownFields))
	for _, field := range knownFields {
		fieldClaims := byField[field]
		slices.SortFunc(fieldClaims, func(a, b Claim) int {
			return cmp.Or(cmp.Compare(a.Source, b.Source), cmp.Compare(a.Value, b.Value))
		})
		out = append(out, Finding{
			Field:     field,
			Status:    statusOf(fieldClaims),
			Staleness: stalenessOf(fieldClaims, now),
			Claims:    fieldClaims,
		})
	}
	return out
}

// statusOf counts distinct values, not claims: two feeds agreeing is stronger
// support, never a conflict.
func statusOf(claims []Claim) Status {
	if len(claims) == 0 {
		return Absent
	}
	values := map[string]bool{}
	for _, c := range claims {
		values[c.Value] = true
	}
	if len(values) > 1 {
		return Conflict
	}
	return Supported
}

// stalenessOf reports on the freshest observation behind the claims, and
// distinguishes "we know it is old" from "no feed told us how old it is".
func stalenessOf(claims []Claim, now time.Time) Staleness {
	var newest time.Time
	for _, c := range claims {
		if c.ObservedAt.After(newest) {
			newest = c.ObservedAt
		}
	}
	if newest.IsZero() {
		return Unknown
	}
	if now.Sub(newest) > FreshWindow {
		return Stale
	}
	return Fresh
}

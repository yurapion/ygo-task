package claimcheck

import (
	"strings"
	"unicode"
)

// nameNoise are words that describe a category of hotel rather than identify
// one, so they must not decide whether two records are the same property.
var nameNoise = map[string]bool{
	"hotel": true, "hotels": true, "resort": true, "spa": true,
	"garni": true, "the": true, "and": true,
}

// group is a property under construction.
type group struct {
	key     string
	name    string
	records []Record
}

// Group collects records that describe the same property, keyed on a collapsed
// name plus the stated city.
//
// The city half of the key is deliberately the city the record states, never
// the one its coordinates imply: grouping has to work before the location has
// been checked, or a record with bad coordinates would be filed under the wrong
// property and its other claims would vanish from the report.
func Group(recs []Record) []Property {
	groups := groupRecords(recs)
	props := make([]Property, 0, len(groups))
	for _, g := range groups {
		props = append(props, g.property())
	}
	return props
}

// groupRecords buckets records by group key, preserving first-seen order so the
// report is stable across runs.
func groupRecords(recs []Record) []*group {
	var order []*group
	index := map[string]*group{}

	for _, rec := range recs {
		key := groupKey(rec)
		g, ok := index[key]
		if !ok {
			g = &group{key: key, name: displayName(rec)}
			index[key] = g
			order = append(order, g)
		}
		g.records = append(g.records, rec)
	}
	return order
}

// property is the group without its claims filled in.
func (g *group) property() Property {
	return Property{
		Key:     g.key,
		Name:    g.name,
		Sources: distinctSources(g.records),
	}
}

// groupKey is the collapsed name and stated city joined.
func groupKey(rec Record) string {
	return collapseName(recordName(rec)) + "|" + strings.ToLower(statedCity(rec))
}

// collapseName reduces a hotel name to an identity that survives the spelling
// drift between feeds: case, punctuation, word order, category words, and
// doubled letters ("Mare Azzurro" against "Mare Azzuro").
//
// Collapsing repeated letters is a blunt instrument. It merges the one
// duplicate this data contains and it would merge genuinely different names
// that differ only by a doubled letter; it is a heuristic standing in for
// entity resolution, not a substitute for it.
func collapseName(name string) string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if !nameNoise[w] {
			words = append(words, w)
		}
	}

	var b strings.Builder
	var prev rune
	for _, r := range strings.Join(words, "") {
		if r != prev {
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

// recordName is whichever name field this feed shape uses.
func recordName(rec Record) string {
	if rec.HotelName != "" {
		return rec.HotelName
	}
	return rec.Name
}

// displayName is what to call the property in the report.
func displayName(rec Record) string {
	if n := recordName(rec); n != "" {
		return n
	}
	if rec.Location != "" {
		return "(unnamed, " + rec.Location + ")"
	}
	return "(unnamed)"
}

// statedCity is the city the record itself asserts, from whichever field shape
// it uses. It does not consult coordinates.
func statedCity(rec Record) string {
	if rec.City != "" {
		return rec.City
	}
	if rec.Location != "" {
		return strings.TrimSpace(strings.SplitN(rec.Location, ",", 2)[0])
	}
	return ""
}

// distinctSources lists each feed once, in first-seen order.
func distinctSources(recs []Record) []string {
	seen := map[string]bool{}
	var out []string
	for _, rec := range recs {
		if rec.Source != "" && !seen[rec.Source] {
			seen[rec.Source] = true
			out = append(out, rec.Source)
		}
	}
	return out
}

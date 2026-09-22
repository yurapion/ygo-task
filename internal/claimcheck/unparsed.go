package claimcheck

import (
	"cmp"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// Counters for what the vocabulary could not place. A facility that was
// discarded and a facility that does not exist look identical in any per-record
// view, so the rate is the thing worth watching: a feed changing its spellings
// shows up here long before it shows up as a wrong answer.
var (
	unparsedFacilities   atomic.Int64
	unparsedDescriptions atomic.Int64
	unknownFields        atomic.Int64
	unparseableDates     atomic.Int64
)

// UnparseableDates is how many records carried a last_seen this decoder could
// not read. Every one of them reports staleness "unknown", which is the safe
// answer and an indistinguishable one: a feed switching to RFC3339 would make
// the whole corpus undateable without a single visible symptom.
func UnparseableDates() int64 { return unparseableDates.Load() }

// UnparsedCounts reports how much of the input went unread, by kind.
func UnparsedCounts() (facilities, descriptions, fields int64) {
	return unparsedFacilities.Load(), unparsedDescriptions.Load(), unknownFields.Load()
}

// countUnparsed tallies one property's unplaceable items.
func countUnparsed(items []Unparsed) {
	for _, u := range items {
		switch u.Kind {
		case "facility":
			unparsedFacilities.Add(1)
		case "description":
			unparsedDescriptions.Add(1)
		case "field":
			unknownFields.Add(1)
		}
	}
}

// recordJSONKeys is every key Record knows how to hold, read off the struct tags
// once so the two can never drift apart. Maintaining a second hand-written list
// would reintroduce exactly the silence this is here to remove.
var recordJSONKeys = sync.OnceValue(func() map[string]bool {
	keys := map[string]bool{}
	t := reflect.TypeFor[Record]()
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			keys[name] = true
		}
	}
	return keys
})

// UnmarshalJSON decodes a record and keeps whatever it could not place.
//
// encoding/json discards unknown keys without a word, so a feed that starts
// publishing a cancellation policy, a resort fee or a new rating scale changes
// nothing visible downstream and nobody finds out until a traveller does. The
// cost of keeping them is one extra decode per record; the cost of not keeping
// them is unbounded and silent.
func (r *Record) UnmarshalJSON(data []byte) error {
	// An alias without Record's methods, so this does not recurse.
	type plain Record
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*r = Record(p)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	known := recordJSONKeys()
	for key, value := range raw {
		if known[key] {
			continue
		}
		// Deliberately not logged here. A feed that adds one field adds it to
		// every record, so a line per key per record is a million lines saying
		// one thing. It is counted, and the count is reported once at the end.
		if r.UnknownFields == nil {
			r.UnknownFields = map[string]string{}
		}
		r.UnknownFields[key] = string(value)
	}
	return nil
}

// unknownFieldUnparsed reports the keys this struct had nowhere to put.
func unknownFieldUnparsed(rec Record) []Unparsed {
	out := make([]Unparsed, 0, len(rec.UnknownFields))
	for key, value := range rec.UnknownFields {
		out = append(out, Unparsed{
			Kind: "field", Name: key, Text: value, Source: rec.Source,
		})
	}
	return out
}

// sortUnparsed orders the report deterministically.
func sortUnparsed(u []Unparsed) {
	slices.SortFunc(u, func(a, b Unparsed) int {
		return cmp.Or(
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Source, b.Source),
			cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.Text, b.Text),
		)
	})
}

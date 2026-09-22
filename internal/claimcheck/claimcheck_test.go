package claimcheck

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var testNow = time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)

func TestCheckCity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		recs []Record
		want Finding
	}{
		{
			// A location no lookup recognises must produce no derived claim at
			// all. Asserting the nearest entry regardless of distance would
			// invent a city and report a conflict that does not exist.
			name: "coords far from every known city derive nothing",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Mid Atlantic Inn",
				City: "Ponta Delgada", Coords: &Coords{Lat: 30.0, Lng: -40.0},
			}},
			want: Finding{
				Field: "city", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "city", Value: "Ponta Delgada", Source: "partner-feed-a/city"},
				},
			},
		},
		{
			// One claim cannot disagree with itself. A derived claim standing
			// alone is support, not conflict.
			name: "coords with no stated city are supported alone",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Nameless Lodge",
				Coords: &Coords{Lat: 48.1351, Lng: 11.5820},
			}},
			want: Finding{
				Field: "city", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "city", Value: "Munich", Source: "derived:coords"},
				},
			},
		},
		{
			// Agreement is counted on values, not on sources. Two feeds saying
			// the same thing is stronger support, never a conflict.
			name: "two sources agreeing are supported not conflicting",
			recs: []Record{
				{Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini"},
				{Source: "scrape-booking-sites", Name: "Mare Azzuro Hotel", Location: "Rimini, Italien"},
			},
			want: Finding{
				Field: "city", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "city", Value: "Rimini", Source: "partner-feed-a/city"},
					{Field: "city", Value: "Rimini", Source: "scrape-booking-sites/location"},
				},
			},
		},
		{
			// Nothing asserted is absent, and absent must still be reported.
			// Dropping the field would hide that the feed never said.
			name: "no city and no coords is absent",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Somewhere Inn",
			}},
			want: Finding{
				Field: "city", Status: Absent, Staleness: Unknown,
			},
		},
		{
			// Case 1. Only one feed speaks, and it contradicts itself: the
			// stated city is Berlin, the coordinates are Munich's, 585km away.
			// Without the derived claim this reports supported and the defect
			// ships.
			name: "stated city conflicts with the city its own coords imply",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "City Lodge Berlin",
				City: "Berlin", Country: "DE",
				Coords: &Coords{Lat: 48.1351, Lng: 11.5820},
			}},
			want: Finding{
				Field: "city", Status: Conflict, Staleness: Unknown,
				Claims: []Claim{
					{Field: "city", Value: "Munich", Source: "derived:coords"},
					{Field: "city", Value: "Berlin", Source: "partner-feed-a/city"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			props := Check(tt.recs, testNow)
			if len(props) != 1 {
				t.Fatalf("got %d properties, want 1", len(props))
			}
			got, ok := props[0].FindingFor("city")
			if !ok {
				t.Fatalf("no finding reported for city; fields present: %v", fields(props[0]))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("finding mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		recs []Record
		want int
	}{
		{
			// Grouping too tight: the two Rimini records stay apart, and the
			// 4-vs-3 star conflict becomes undetectable because there is
			// nothing to compare against.
			name: "same hotel spelled differently is one property",
			recs: []Record{
				{Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini"},
				{Source: "scrape-booking-sites", Name: "Mare Azzuro Hotel", Location: "Rimini, Italien"},
			},
			want: 1,
		},
		{
			// Grouping too loose: two real hotels merge and every field between
			// them is reported as a conflict that does not exist.
			name: "different hotels in one city stay separate",
			recs: []Record{
				{Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini"},
				{Source: "partner-feed-a", HotelName: "Grand Hotel Rimini", City: "Rimini"},
			},
			want: 2,
		},
		{
			// A chain name in two cities is two properties. Name alone is not a
			// key.
			name: "same name in different cities stays separate",
			recs: []Record{
				{Source: "partner-feed-a", HotelName: "City Lodge", City: "Berlin"},
				{Source: "partner-feed-a", HotelName: "City Lodge", City: "Hamburg"},
			},
			want: 2,
		},
		{
			// An unnamed record is still a record. Dropping it loses whatever it
			// claims without saying so.
			name: "record with no name is kept",
			recs: []Record{
				{Source: "scrape-booking-sites", Location: "Rimini, Italien"},
			},
			want: 1,
		},
		{
			name: "no records is no properties",
			recs: nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Group(tt.recs)
			if len(got) != tt.want {
				names := make([]string, 0, len(got))
				for _, p := range got {
					names = append(names, p.Name)
				}
				t.Errorf("got %d properties %v, want %d", len(got), names, tt.want)
			}
		})
	}
}

func fields(p Property) []string {
	out := make([]string, 0, len(p.Findings))
	for _, f := range p.Findings {
		out = append(out, f.Field)
	}
	return out
}

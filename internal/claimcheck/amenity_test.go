package claimcheck

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func strptr(s string) *string { return &s }

// TestCheckAmenity covers the fourth defect: an amenity advertised by the
// partner feed and contradicted by a review the scrape collected. The data
// contains its own counter-evidence, and a merge that unions amenity lists
// destroys it.
func TestCheckAmenity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		recs  []Record
		field string
		want  Finding
	}{
		{
			// Alpenhof ships amenities: null. Unknown is not "no pool" — an
			// agent that reports it as false rules the hotel out of a search it
			// might well have matched.
			name: "null amenities assert nothing",
			recs: []Record{{
				Source: "partner-feed-b", HotelName: "Alpenhof Garni", City: "Innsbruck",
				Amenities: nil,
			}},
			field: "amenity.pool",
			want:  Finding{Field: "amenity.pool", Status: Absent, Staleness: Unknown},
		},
		{
			name: "an empty amenities string asserts nothing",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Bare Inn", City: "Berlin",
				Amenities: strptr(""),
			}},
			field: "amenity.pool",
			want:  Finding{Field: "amenity.pool", Status: Absent, Staleness: Unknown},
		},
		{
			// A review that merely mentions an amenity is not a contradiction.
			// Only a closure is.
			name: "a positive review does not contradict the amenity",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Sunny Inn", City: "Rimini",
				Amenities:      strptr("pool"),
				ReviewSnippets: []string{"the pool was great", "lovely staff"},
			}},
			field: "amenity.pool",
			want: Finding{
				Field: "amenity.pool", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "amenity.pool", Value: "true", Source: "partner-feed-a/amenities"},
				},
			},
		},
		{
			// A closure with no amenity named asserts nothing about the pool.
			name: "a closure naming another amenity does not touch this one",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Sunny Inn", City: "Rimini",
				Amenities:      strptr("pool"),
				ReviewSnippets: []string{"the restaurant was closed all week"},
			}},
			field: "amenity.pool",
			want: Finding{
				Field: "amenity.pool", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "amenity.pool", Value: "true", Source: "partner-feed-a/amenities"},
				},
			},
		},
		{
			// The feeds spell the same facility three ways. A filter keyed on
			// the literal string silently drops two of them.
			name: "a quantified feature is the same amenity as the bare word",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Sunset Bay", Location: "Playa del Carmen, Mexico",
				Features: []string{"3 pools", "free WiFi"},
			}},
			field: "amenity.pool",
			want: Finding{
				Field: "amenity.pool", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "amenity.pool", Value: "true", Source: "scrape-booking-sites/features"},
				},
			},
		},
		{
			// Case 4. The pool is advertised by one feed and reported closed by
			// the other, and the evidence for the closure is nearly three years
			// old. Both facts have to reach the reader: the conflict and the
			// age of the source it rests on.
			name: "the advertised pool conflicts with a stale review reporting it closed",
			recs: []Record{
				{
					Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini",
					Amenities: strptr("pool,wifi,parking"),
				},
				{
					Source: "scrape-booking-sites", Name: "Mare Azzuro Hotel", Location: "Rimini, Italien",
					Features:       []string{"swimming pool", "free WiFi", "pets allowed"},
					ReviewSnippets: []string{"pool was closed for the whole of August", "great breakfast"},
					LastSeen:       "2023-11-02",
				},
			},
			field: "amenity.pool",
			want: Finding{
				Field: "amenity.pool", Status: Conflict, Staleness: Stale,
				Claims: []Claim{
					{Field: "amenity.pool", Value: "true", Source: "partner-feed-a/amenities"},
					{Field: "amenity.pool", Value: "true", Source: "scrape-booking-sites/features", ObservedAt: mustDate("2023-11-02")},
					{Field: "amenity.pool", Value: "false", Source: "scrape-booking-sites/review_snippets", ObservedAt: mustDate("2023-11-02")},
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
			got, ok := props[0].FindingFor(tt.field)
			if !ok {
				t.Fatalf("no finding reported for %s; fields present: %v", tt.field, fields(props[0]))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("finding mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

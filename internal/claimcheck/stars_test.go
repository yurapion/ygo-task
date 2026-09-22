package claimcheck

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestCheckStars covers the third defect: two feeds describing one hotel with
// different star ratings. It only exists as a conflict because grouping merged
// the pair first — ungrouped, there is no disagreement, just two hotels.
//
// The rows that matter most are the ones that refuse to make a claim. A star
// classification and a guest score are different quantities, and a scrape's
// "rating" field can hold either; reading a 8.4 out of 10 as an 8-star hotel is
// a category error that is invisible once it is a number in a column.
func TestCheckStars(t *testing.T) {
	t.Parallel()

	four := 4
	zero := 0
	six := 6

	tests := []struct {
		name string
		recs []Record
		want Finding
	}{
		{
			// City Lodge Berlin ships stars: null. Null is not zero stars and
			// not one star; it is the feed declining to say.
			name: "null stars is absent",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "City Lodge Berlin", City: "Berlin",
				Stars: nil,
			}},
			want: Finding{Field: "stars", Status: Absent, Staleness: Unknown},
		},
		{
			name: "a decimal guest score is not a star classification",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Scored Hotel", Location: "Rimini, Italien",
				Rating: "8.4",
			}},
			want: Finding{Field: "stars", Status: Absent, Staleness: Unknown},
		},
		{
			name: "a score out of ten is not a star classification",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Scored Hotel", Location: "Rimini, Italien",
				Rating: "9/10",
			}},
			want: Finding{Field: "stars", Status: Absent, Staleness: Unknown},
		},
		{
			name: "zero stars is outside the classification and asserts nothing",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Unrated Inn", City: "Berlin",
				Stars: &zero,
			}},
			want: Finding{Field: "stars", Status: Absent, Staleness: Unknown},
		},
		{
			name: "six stars is outside the classification and asserts nothing",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Overrated Inn", City: "Berlin",
				Stars: &six,
			}},
			want: Finding{Field: "stars", Status: Absent, Staleness: Unknown},
		},
		{
			name: "two feeds agreeing on four stars is supported",
			recs: []Record{
				{Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini", Stars: &four},
				{Source: "scrape-booking-sites", Name: "Mare Azzuro Hotel", Location: "Rimini, Italien", Rating: "4 stars"},
			},
			want: Finding{
				Field: "stars", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "stars", Value: "4", Source: "partner-feed-a/stars"},
					{Field: "stars", Value: "4", Source: "scrape-booking-sites/rating"},
				},
			},
		},
		{
			// Case 3. Neither wins. A traveller told "4 stars" by a merge that
			// picked the partner feed has been sold a grade the other source
			// disputes.
			name: "four stars conflicts with three on the grouped Rimini pair",
			recs: []Record{
				{Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini", Stars: &four},
				{Source: "scrape-booking-sites", Name: "Mare Azzuro Hotel", Location: "Rimini, Italien", Rating: "3 stars", LastSeen: "2023-11-02"},
			},
			want: Finding{
				Field: "stars", Status: Conflict, Staleness: Stale,
				Claims: []Claim{
					{Field: "stars", Value: "4", Source: "partner-feed-a/stars"},
					{Field: "stars", Value: "3", Source: "scrape-booking-sites/rating", ObservedAt: mustDate("2023-11-02")},
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
			got, ok := props[0].FindingFor("stars")
			if !ok {
				t.Fatalf("no finding reported for stars; fields present: %v", fields(props[0]))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("finding mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

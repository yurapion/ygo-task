package claimcheck

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestCheckChildPolicy covers the second class of defect in this feed: a policy
// asserted in prose and contradicted by a structured field on the same record.
// Whichever a reader believes, the other one books the wrong holiday.
func TestCheckChildPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		recs []Record
		want Finding
	}{
		{
			// Silence is not a policy. A hotel that never mentions children is
			// not thereby adults-only, and must not be reported as either.
			name: "no description and no features is absent",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Quiet Inn", City: "Berlin",
			}},
			want: Finding{Field: "policy.children", Status: Absent, Staleness: Unknown},
		},
		{
			// Features unrelated to children must not be read as a signal
			// either way. A substring match that fires on "pool" would.
			name: "features with no child signal is absent",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Quiet Inn", Location: "Berlin, DE",
				Description: "A calm place to stay.",
				Features:    []string{"spa", "swimming pool", "parking"},
			}},
			want: Finding{Field: "policy.children", Status: Absent, Staleness: Unknown},
		},
		{
			// An affirmative no is support, not absence. This is the value the
			// report must be able to state out loud.
			name: "adults-only with no contradicting feature is supported false",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Adult Retreat", Location: "Rimini, Italien",
				Description: "An Adults-Only sanctuary of tranquility.",
				Features:    []string{"spa", "swim-up bar"},
			}},
			want: Finding{
				Field: "policy.children", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "policy.children", Value: "false", Source: "scrape-booking-sites/description"},
				},
			},
		},
		{
			name: "a kids club with no contradicting prose is supported true",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Family Resort", Location: "Rimini, Italien",
				Description: "A resort for everyone.",
				Features:    []string{"kids club", "spa"},
			}},
			want: Finding{
				Field: "policy.children", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "policy.children", Value: "true", Source: "scrape-booking-sites/features"},
				},
			},
		},
		{
			// Case 2. One record, two fields, opposite policies. Both claims
			// come from the same feed, so anything keyed on the feed alone sees
			// no disagreement here.
			name: "adults-only prose conflicts with a kids club on the same record",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Sunset Bay Resort & Spa",
				Location:    "Playa del Carmen, Mexico",
				Description: "An adults-only sanctuary of tranquility.",
				Features:    []string{"spa", "3 pools", "swim-up bar", "kids club"},
			}},
			want: Finding{
				Field: "policy.children", Status: Conflict, Staleness: Unknown,
				Claims: []Claim{
					{Field: "policy.children", Value: "false", Source: "scrape-booking-sites/description"},
					{Field: "policy.children", Value: "true", Source: "scrape-booking-sites/features"},
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
			got, ok := props[0].FindingFor("policy.children")
			if !ok {
				t.Fatalf("no finding reported for policy.children; fields present: %v", fields(props[0]))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("finding mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

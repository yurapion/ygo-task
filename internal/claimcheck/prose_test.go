package claimcheck

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestCheckAlpenhofProse covers the case that proves the status vocabulary is
// honest. Alpenhof Garni ships amenities: null, and states in German prose that
// breakfast is included and that there is no restaurant.
//
// Three different answers have to come out of one record: a facility affirmed,
// a facility denied, and a facility nobody mentioned. Collapsing any two of
// them is how a normaliser tells a traveller there is no breakfast because the
// feed's structured field happened to be null.
func TestCheckAlpenhofProse(t *testing.T) {
	t.Parallel()

	alpenhof := []Record{{
		Source: "partner-feed-b", HotelName: "Alpenhof Garni",
		City: "Innsbruck", Country: "AT",
		Description: "Familiengeführtes Hotel im Herzen der Alpen. Kein Restaurant, Frühstück inklusive.",
		Amenities:   nil,
		Coords:      &Coords{Lat: 47.2692, Lng: 11.4041},
	}}

	tests := []struct {
		name  string
		field string
		want  Finding
	}{
		{
			// The denial. This must not print the same as "absent", and it must
			// not be lost because the structured field was null.
			name:  "a facility denied in prose is supported false",
			field: "amenity.restaurant",
			want: Finding{
				Field: "amenity.restaurant", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "amenity.restaurant", Value: "false", Source: "partner-feed-b/description"},
				},
			},
		},
		{
			// The affirmation, in a language the structured schema never reads.
			name:  "a facility affirmed in prose is supported true",
			field: "amenity.breakfast",
			want: Finding{
				Field: "amenity.breakfast", Status: Supported, Staleness: Unknown,
				Claims: []Claim{
					{Field: "amenity.breakfast", Value: "true", Source: "partner-feed-b/description"},
				},
			},
		},
		{
			// The silence. Null amenities plus no mention is absent, which is
			// not the same answer as the restaurant's false.
			name:  "a facility nobody mentions is absent, not false",
			field: "amenity.pool",
			want:  Finding{Field: "amenity.pool", Status: Absent, Staleness: Unknown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			props := Check(alpenhof, testNow)
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

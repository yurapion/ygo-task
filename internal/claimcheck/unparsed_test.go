package claimcheck

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestUnknownJSONFields covers a feed adding a field this struct has never seen.
// Left to encoding/json it is discarded in silence, which means a new feed can
// change what it publishes and nothing downstream ever finds out.
func TestUnknownJSONFields(t *testing.T) {
	t.Parallel()

	const feed = `[{
		"source": "partner-feed-c",
		"hotel_name": "New Feed Hotel",
		"city": "Berlin",
		"cancellation_policy": "free until 24h before arrival",
		"loyalty_tier": 3
	}]`

	recs, err := LoadRecords(strings.NewReader(feed))
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}

	props := Check(recs, testNow)
	if len(props) != 1 {
		t.Fatalf("got %d properties, want 1", len(props))
	}

	want := []Unparsed{
		{Kind: "field", Name: "cancellation_policy", Text: `"free until 24h before arrival"`, Source: "partner-feed-c"},
		{Kind: "field", Name: "loyalty_tier", Text: "3", Source: "partner-feed-c"},
	}
	if diff := cmp.Diff(want, props[0].Unparsed); diff != "" {
		t.Errorf("unparsed mismatch (-want +got):\n%s", diff)
	}
}

// TestUnparsedFacilities covers the tokens the controlled vocabulary does not
// recognise. Dropping them is what makes a vocabulary look complete: the report
// would show a hotel's amenities as fully understood while two of them were
// thrown away.
//
// "pets allowed" is a policy and "swim-up bar" is a facility with no canonical
// name, and nothing here can tell them apart. Separating them needs the
// vocabulary this bucket exists to avoid guessing at, so both land here as raw
// text and a reader decides.
func TestUnparsedFacilities(t *testing.T) {
	t.Parallel()

	recs := []Record{{
		Source: "scrape-booking-sites", Name: "Sunset Bay Resort & Spa",
		Location:  "Playa del Carmen, Mexico",
		Features:  []string{"spa", "3 pools", "swim-up bar", "pets allowed"},
		Amenities: strptr("pool,,minibar"),
	}}

	props := Check(recs, testNow)
	if len(props) != 1 {
		t.Fatalf("got %d properties, want 1", len(props))
	}

	// "spa", "3 pools" and "pool" map to the vocabulary; the empty CSV token is
	// nothing at all and must not be reported as an unrecognised facility.
	want := []Unparsed{
		{Kind: "facility", Text: "minibar", Source: "scrape-booking-sites/amenities"},
		{Kind: "facility", Text: "pets allowed", Source: "scrape-booking-sites/features"},
		{Kind: "facility", Text: "swim-up bar", Source: "scrape-booking-sites/features"},
	}
	if diff := cmp.Diff(want, props[0].Unparsed); diff != "" {
		t.Errorf("unparsed mismatch (-want +got):\n%s", diff)
	}
}

// TestUnparsedDescription covers prose that no signal matched.
//
// Only a description where nothing fired is reported. A description that fired
// at least one signal was read, even if imperfectly, and printing every one of
// those would bury the conflicts under prose that was never going to map to a
// field.
func TestUnparsedDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		recs []Record
		want []Unparsed
	}{
		{
			name: "prose matching no signal is reported with its raw text",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "Hotel Mare Azzurro", City: "Rimini",
				Description: "<b>WELCOME TO PARADISE!!!</b> Best hotel in Rimini!",
			}},
			want: []Unparsed{{
				Kind:   "description",
				Text:   "<b>WELCOME TO PARADISE!!!</b> Best hotel in Rimini!",
				Source: "partner-feed-a/description",
			}},
		},
		{
			name: "prose that fired a signal is not reported",
			recs: []Record{{
				Source: "scrape-booking-sites", Name: "Sunset Bay", Location: "Playa del Carmen, Mexico",
				Description: "An adults-only sanctuary of tranquility.",
			}},
			want: nil,
		},
		{
			// City Lodge Berlin ships description: "". There is no prose here to
			// have missed, and reporting it would be noise standing in for a gap.
			name: "an empty description is not unparsed prose",
			recs: []Record{{
				Source: "partner-feed-a", HotelName: "City Lodge Berlin", City: "Berlin",
				Description: "",
			}},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			props := Check(tt.recs, testNow)
			if len(props) != 1 {
				t.Fatalf("got %d properties, want 1", len(props))
			}
			if diff := cmp.Diff(tt.want, props[0].Unparsed); diff != "" {
				t.Errorf("unparsed mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

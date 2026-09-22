package claimcheck

import "strings"

// canonicalAmenities is the controlled vocabulary the report speaks. It is
// fixed rather than discovered, because "absent" is only meaningful relative to
// a list of things we know to ask about: a facility nobody ever names would
// otherwise be indistinguishable from a facility nobody has.
var canonicalAmenities = []string{"pool", "wifi", "parking", "spa", "restaurant", "breakfast"}

// amenityAliases map the spellings the feeds actually use onto that vocabulary.
// The same facility appears here as "pool", "swimming pool" and "3 pools"; a
// filter keyed on the literal string silently drops two of the three.
var amenityAliases = map[string]string{
	"pool": "pool", "swimming pool": "pool", "pools": "pool", "swimming pools": "pool",
	"wifi": "wifi", "wi-fi": "wifi", "free wifi": "wifi", "free wi-fi": "wifi", "internet": "wifi",
	"parking": "parking", "free parking": "parking", "car park": "parking",
	"spa": "spa", "wellness": "spa",
	"restaurant": "restaurant",
	"breakfast":  "breakfast", "breakfast included": "breakfast",
}

// closureWords mark a review as reporting a facility unavailable rather than
// merely mentioning it. Without them, "the pool was great" would be read as a
// contradiction of the pool.
var closureWords = []string{"closed", "out of order", "not working", "unavailable", "under renovation", "shut"}

// amenityClaims are what a record's amenity list, feature list and review
// snippets each assert about the controlled vocabulary.
//
// A review is a source like any other. It is weaker evidence than a partner
// feed in most people's judgement, but nothing here ranks them: the report
// states that one says the pool exists and the other says it was closed, and
// leaves the reader to weigh a three-year-old review against an undated feed.
func amenityClaims(rec Record) ([]Claim, []Unparsed) {
	c := claimSet{observed: observedAt(rec)}
	var unparsed []Unparsed

	// A nil amenities field is the feed declining to say. Only a non-nil
	// string is evidence, and an empty one still asserts nothing.
	if rec.Amenities != nil {
		for _, token := range strings.Split(*rec.Amenities, ",") {
			if name, ok := canonicalAmenity(token); ok {
				c.add("amenity."+name, "true", rec.Source+"/amenities")
				continue
			}
			unparsed = appendUnparsedFacility(unparsed, token, rec.Source+"/amenities")
		}
	}

	// A feature can be a facility ("3 pools") or a policy signal ("kids club"),
	// and it is only unrecognised when it is neither. Checking one and not the
	// other would report half the vocabulary's own hits as misses.
	for _, feature := range rec.Features {
		matched := false
		if name, ok := canonicalAmenity(feature); ok {
			c.add("amenity."+name, "true", rec.Source+"/features")
			matched = true
		}
		lowered := strings.ToLower(feature)
		for _, sig := range facilitySignals {
			if sig.fires(lowered) {
				c.add(sig.field, sig.value, rec.Source+"/features")
				matched = true
			}
		}
		if !matched {
			unparsed = appendUnparsedFacility(unparsed, feature, rec.Source+"/features")
		}
	}

	for _, snippet := range rec.ReviewSnippets {
		lowered := strings.ToLower(snippet)
		if !containsAny(lowered, closureWords) {
			continue
		}
		for _, name := range canonicalAmenities {
			if strings.Contains(lowered, name) {
				c.add("amenity."+name, "false", rec.Source+"/review_snippets")
			}
		}
	}

	return c.claims, unparsed
}

// appendUnparsedFacility records a token the vocabulary does not recognise.
//
// The bucket deliberately mixes two things it cannot separate: a facility with
// no canonical name ("swim-up bar") and a token that is not a facility at all
// ("pets allowed"). Telling them apart needs the vocabulary this exists to stop
// guessing at, so both are handed to the reader as raw text.
func appendUnparsedFacility(out []Unparsed, raw, source string) []Unparsed {
	text := strings.TrimSpace(raw)
	if text == "" {
		// An empty CSV token is not an unrecognised facility, it is nothing.
		return out
	}
	return append(out, Unparsed{Kind: "facility", Text: text, Source: source})
}

// canonicalAmenity maps one feed spelling onto the vocabulary, dropping any
// leading quantity so "3 pools" and "pool" are the same facility.
func canonicalAmenity(raw string) (string, bool) {
	token := strings.TrimSpace(strings.ToLower(raw))
	token = strings.TrimLeft(token, "0123456789 ")
	name, ok := amenityAliases[token]
	return name, ok
}

// containsAny reports whether s contains any of the needles.
func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// amenityFields are the known fields contributed by the vocabulary.
func amenityFields() []string {
	out := make([]string, 0, len(canonicalAmenities))
	for _, name := range canonicalAmenities {
		out = append(out, "amenity."+name)
	}
	return out
}

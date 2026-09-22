package claimcheck

import (
	"regexp"
	"strconv"
)

// starRange is the official hotel classification, which is what the partner
// feeds' "stars" field means. A value outside it is not a lower or higher
// grade, it is a field being used for something else.
const (
	minStars = 1
	maxStars = 5
)

// starPhrase matches a rating that names its own unit, e.g. "3 stars".
//
// A scrape's "rating" field can hold a star classification or a guest score,
// and the two are different quantities on different scales. Only a value that
// says "star" is read as a classification; "8.4" and "9/10" assert nothing
// here, because reading a guest score as a star count is a category error that
// becomes invisible the moment it is a number in a column.
var starPhrase = regexp.MustCompile(`^\s*([1-5])\s*stars?\s*$`)

// starClaims are what a record asserts about the property's star
// classification, from whichever field shape its feed uses.
func starClaims(rec Record) []Claim {
	c := claimSet{observed: observedAt(rec)}

	// A nil Stars is the feed declining to say. It is not zero stars, and
	// defaulting it to a number is how "we don't know" becomes "it's bad".
	if rec.Stars != nil && *rec.Stars >= minStars && *rec.Stars <= maxStars {
		c.add("stars", strconv.Itoa(*rec.Stars), rec.Source+"/stars")
	}

	if m := starPhrase.FindStringSubmatch(rec.Rating); m != nil {
		c.add("stars", m[1], rec.Source+"/rating")
	}

	return c.claims
}

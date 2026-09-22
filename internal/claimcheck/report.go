package claimcheck

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Report writes every property with every claim, its source, and what the
// claims add up to.
//
// A conflict prints every competing value side by side and stops there. No feed
// outranks another, the newest does not win, and the majority does not win:
// nothing in this data justifies any of those rules, and a single merged value
// would carry the authority of a schema while being a guess.
func Report(w io.Writer, props []Property) error {
	for i, p := range props {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := reportProperty(w, p); err != nil {
			return err
		}
	}
	return nil
}

func reportProperty(w io.Writer, p Property) error {
	if _, err := fmt.Fprintf(w, "%s\n  sources: %s\n", p.Name, strings.Join(p.Sources, ", ")); err != nil {
		return err
	}
	for _, f := range p.Findings {
		if _, err := fmt.Fprintf(w, "  %-20s %-10s %s\n",
			f.Field, strings.ToUpper(string(f.Status)), f.Staleness); err != nil {
			return err
		}
		for _, c := range f.Claims {
			if _, err := fmt.Fprintf(w, "      %-22s %-36s %s\n",
				quoted(c.Value), c.Source, observedLabel(c)); err != nil {
				return err
			}
		}
	}
	return reportUnparsed(w, p.Unparsed)
}

// reportUnparsed prints what the feed published and this tool could not place.
//
// It is the section that keeps the rest honest. Every status above is computed
// over a vocabulary someone chose, and without this block a facility that was
// discarded and a facility that does not exist produce the same output.
func reportUnparsed(w io.Writer, unparsed []Unparsed) error {
	if len(unparsed) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "  %-20s %d\n", "UNPARSED", len(unparsed)); err != nil {
		return err
	}
	for _, u := range unparsed {
		if _, err := fmt.Fprintf(w, "      %-12s %-45s %s\n",
			u.Kind, unparsedText(u), u.Source); err != nil {
			return err
		}
	}
	return nil
}

// unparsedText renders a JSON key beside its raw value, and everything else as
// the raw text itself.
func unparsedText(u Unparsed) string {
	if u.Name != "" {
		return u.Name + " = " + truncate(u.Text, 45-len(u.Name)-3)
	}
	// Truncate inside the quotes, so a clipped line still reads as a complete
	// quoted string rather than one missing its closing mark.
	return quoted(truncate(u.Text, 43))
}

// truncate keeps one unparsed item to one line. A description can be a
// paragraph, and a report nobody can scan hides the conflicts as effectively as
// not printing them.
//
// The limit counts runes, not bytes. These feeds carry German and Spanish prose,
// and a byte slice lands inside a multi-byte rune often enough that the report
// would print mojibake exactly where it is quoting text nobody parsed.
func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max-1]) + "…"
}

// observedLabel keeps "the feed carried no date" visually distinct from a date,
// so an undated claim is never read as a current one.
func observedLabel(c Claim) string {
	if c.ObservedAt.IsZero() {
		return "no date"
	}
	return c.ObservedAt.Format(lastSeenLayout)
}

// quoted wraps a value so an empty string is visible in the report rather than
// printing as blank space.
func quoted(s string) string { return `"` + s + `"` }

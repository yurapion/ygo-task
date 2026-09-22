package claimcheck

import (
	"fmt"
	"io"
	"strings"
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
	return nil
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

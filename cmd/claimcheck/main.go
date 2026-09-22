// Command claimcheck reports what each feed claims about each hotel property,
// and where those claims disagree.
package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/yuratestdev/ygo-task/internal/claimcheck"
)

func main() {
	// Structured from the start: these lines are the only machine-readable view
	// of how much of a feed went unread, and an interpolated message cannot be
	// grouped or alerted on.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	if err := run(os.Args[1:], os.Stdout); err != nil {
		slog.Error("claimcheck failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: claimcheck <hotels.json>")
	}

	f, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("open feed: %w", err)
	}
	// Read-only handle; a Close error cannot affect the report already written.
	defer func() { _ = f.Close() }()

	recs, err := claimcheck.LoadRecords(f)
	if err != nil {
		return fmt.Errorf("decode feed: %w", err)
	}

	props := claimcheck.Check(recs, time.Now())
	if err := claimcheck.Report(out, props); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	// One summary line per run, always. A run with nothing wrong still has to
	// say how much it looked at, or "no conflicts" is indistinguishable from
	// "read nothing".
	slog.Info("report written",
		"records", len(recs), "properties", len(props), "conflicts", countConflicts(props))

	if n := claimcheck.UnresolvedCoords(); n > 0 {
		slog.Warn("some coordinates were never cross-checked", "count", n)
	}
	if facilities, descriptions, fields := claimcheck.UnparsedCounts(); facilities+descriptions+fields > 0 {
		slog.Warn("some of the feed went unread",
			"facilities", facilities, "descriptions", descriptions, "unknown_fields", fields)
	}
	if n := claimcheck.UnparseableDates(); n > 0 {
		slog.Warn("some records carried a last_seen this decoder cannot read", "count", n)
	}
	return nil
}

// countConflicts is the headline number: how many fields have sources that
// disagree and were left unresolved.
func countConflicts(props []claimcheck.Property) int {
	n := 0
	for _, p := range props {
		for _, f := range p.Findings {
			if f.Status == claimcheck.Conflict {
				n++
			}
		}
	}
	return n
}

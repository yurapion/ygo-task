// Command claimcheck reports what each feed claims about each hotel property,
// and where those claims disagree.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/yuratestdev/ygo-task/internal/claimcheck"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		slog.Error("claimcheck failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: claimcheck <hotels.json>")
	}

	f, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("open feed: %w", err)
	}
	defer f.Close()

	recs, err := claimcheck.LoadRecords(f)
	if err != nil {
		return fmt.Errorf("decode feed: %w", err)
	}

	props := claimcheck.Check(recs, time.Now())
	if err := claimcheck.Report(out, props); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if n := claimcheck.UnresolvedCoords(); n > 0 {
		slog.Warn("some coordinates were never cross-checked", "count", n)
	}
	return nil
}

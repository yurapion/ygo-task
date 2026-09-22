# claimcheck

Reads a hotel feed assembled from several sources and reports what each source
claims about each property, and where those claims disagree.

```bash
go run ./cmd/claimcheck testdata/hotels.json
```

Records describing the same hotel are grouped into one property, so two feeds
spelling it "Hotel Mare Azzurro" and "Mare Azzuro Hotel" are compared rather than
listed twice. Each field then reports a status and, separately, how old the
evidence behind it is:

```
Hotel Mare Azzurro
  sources: partner-feed-a, scrape-booking-sites
  stars                CONFLICT   stale
      "4"                    partner-feed-a/stars                 no date
      "3"                    scrape-booking-sites/rating          2023-11-02
```

`supported` means the sources agree — including agreeing that something is *not*
the case. `absent` means nobody said. They are different answers, and a null
field is the second one, never the first.

**On a conflict the report prints every value with its source and stops.** It does
not rank feeds, prefer the newest, or take a majority. A single merged value would
carry the authority of a schema while being a guess, and the guesses that matter
here send someone to the wrong city.

The design, what was deliberately left out, and the honest list of what is weak
are in [DECISIONS.md](DECISIONS.md).

```bash
go test ./... -race -count=1
```

# Decisions

## The feature, and who it helps

A **claim check** over a hotel feed from mixed sources.

Records are grouped into *properties* (the two Rimini records, "Hotel Mare Azzurro"
from a partner feed and "Mare Azzuro Hotel" from a scrape, are one property). For
each property the tool prints every **claim** — one asserted fact about that
property — together with the sources that assert it and a status.

A claim is `(property, field, value, source, observed_at)`. Two things about that
shape carry most of the weight:

- **The source key is `(feed, field-path)`, not just `feed`.** The worst errors in
  this data are a single record disagreeing with *itself*: one partner record says
  `city: "Berlin"` and `coords: 48.1351, 11.5820`, which is Munich; one scrape says
  `description: "adults-only"` and `features: [... "kids club"]`. If claims were
  attributed per-feed, both would read as a single uncontested source and pass.
- **Coordinates emit a derived claim.** `coords` are resolved to a nearest known
  city and asserted as a `city` claim from `derived:coords`, so a location that is
  internally inconsistent becomes an ordinary conflict between two claims rather
  than a thing no comparison can see.

Status is two independent axes, because one property is routinely both things at
once:

| `status` | meaning |
|---|---|
| `supported` | one or more sources assert it, and they agree (the value may be `false` — an affirmative *no*) |
| `conflict` | two or more sources assert incompatible values |
| `absent` | no source asserts anything about this field |

| `staleness` | meaning |
|---|---|
| `fresh` | newest supporting observation is within the freshness window |
| `stale` | newest supporting observation is older than the window |
| `unknown` | no source carries a timestamp at all |

`absent` and `supported (false)` are kept apart deliberately. `Alpenhof Garni` has
`amenities: null` and a German description reading `"Kein Restaurant"` — "nobody
said" and "a source said no" are different facts, and collapsing them is how a
normaliser tells a traveller a hotel has no breakfast when the feed simply never
mentioned breakfast.

**On conflict the tool prints both values and both sources and stops.** It does not
rank feeds, take the newest, or take the majority. There is no evidence in this data
that would justify any of those rules, and a merged record that silently picked one
is more dangerous than two records that disagree, because it has the authority of a
schema behind it.

Who it helps: the person downstream of the feed — an agent, or the engineer wiring
one up — who otherwise gets a clean-looking record and sends a traveller 585 km to
the wrong city, or books a family into an adults-only resort. Every defect this
catches is one that parses correctly and looks right.

## Two alternatives dropped

**A normalise-and-merge pipeline.** The obvious read of "unify these feeds": map
`name`/`hotel_name` to one field, split the amenity CSV, coerce the types, emit one
clean record per property. Dropped because it is actively worse than doing nothing
on the four defects that matter. Every one of them survives normalisation intact and
comes out the far side wearing a validated schema: the Munich coordinates are still
Munich, the adults-only resort still has a kids club, and the 3-vs-4-star conflict
gets resolved by whichever record the map iteration happened to write last. It makes
bad data harder to distrust. Roughly 20 minutes to build and it would have answered
the brief's letter.

**Currency and price normalisation.** `price_from_eur: 89` against
`price_from: "180 USD"` is a real defect — a sort by price mixes units and means
nothing. Dropped because the claim check structurally cannot express it: the two
prices are asserted about *different properties*, so there is no conflict between
claims to detect. It is a cross-property comparability problem, and bending the
claim model to reach it would cost more than it returns inside this scope. It needs
an FX rate with an as-of date, plus a stay date, occupancy and tax basis that this
data does not carry, before a converted number would be honest. Listed under Known
gaps instead.

## How we'll know it works

Five acceptance cases, built one at a time, with the full report printed after each
so the output is legible at every step. Each starts as a failing test.

1. **Berlin against the Munich coordinates.** The `city` field on City Lodge Berlin
   reports `conflict`: `"Berlin"` from `partner-feed-a/city`, `"Munich"` from
   `derived:coords`. Without the derived claim this case reports `supported` — that
   is the specific regression the test pins.
2. **Adults-only against the kids club.** Sunset Bay reports a conflict between
   `partner.../description` and `partner.../features`, two claims from one record.
3. **Four stars against three.** The two Rimini records group into one property, and
   `stars` reports `conflict`: `4` from `partner-feed-a`, `3` from
   `scrape-booking-sites`. Neither wins. Grouping is what makes this case exist at
   all — ungrouped, there is no disagreement, just two hotels.
4. **The pool against the August review, and the 2023 timestamp.** `pool` reports
   `conflict` (asserted as an amenity, contradicted by a review snippet) *and*
   `staleness: stale` on the same line, from `last_seen: 2023-11-02`. A single-enum
   status cannot report both; this case is what forces the two axes apart.
5. **Null amenities against the German description.** Alpenhof reports
   `breakfast: supported (true)` and `restaurant: supported (false)` — both from
   prose — while a field nothing asserts reports `absent`. `null` must not print as
   `false`, and the two must not print the same.

The gate for each: `gofmt -l . && go vet ./... && go test ./... -race -count=1`.

### Known gaps

All five acceptance cases are green and every guard below was checked by deleting
it and naming the test that went red. What follows is what I know is weak.

**The text matching is the weakest thing in here.** Signals are case-insensitive
substring tests. That is enough to surface a contradiction the feed already
contains and nothing more. It bit immediately: `"Kein Restaurant"` contains
`"restaurant"`, so the positive and negative signals both fired on one
unambiguous sentence and the report invented a conflict inside it. Positive
signals now name the phrases that contain them while meaning the opposite, which
is a patch, not a fix — the general problem is negation and it needs a parser or a
model, not a longer list. A policy stated in a phrasing not in the table is missed
silently, and a missed signal looks exactly like an absent one.

- **Price and currency comparability** is not attempted, for the reason above.
- **Nearest-city resolution is a fixed six-entry lookup, not a geocoder.** Any
  coordinate more than 50km from an entry gets no cross-check at all, which is a
  silent degradation: an unchecked location reports the same as a checked one that
  agreed. It is logged and counted (`UnresolvedCoords`) precisely so a lookup that
  has fallen behind the feed shows up as a rate rather than as quiet.
- **Property grouping is a heuristic**: collapsed name plus stated city, where
  collapsing drops category words and repeated letters. That merges
  Azzurro/Azzuro, and it would merge two real hotels whose names differ only by a
  doubled letter. It is not entity resolution. A record with no name keys on the
  city alone and would collect other unnamed records in that city.
- **Grouping uses the stated city, so a record with wrong coordinates groups
  correctly but a record with a wrong *city* would not.** City Lodge Berlin is the
  first case; the mirror image is not handled.
- **`policy.children` reads the features array but not the amenities CSV.** A feed
  putting "kids club" in the CSV would be missed. No record here does.
- **`cmd/claimcheck` has no tests**, and `LoadRecords` is untested against
  malformed JSON — it returns the decoder's error unwrapped.
- **`UnresolvedCoords` is a process-level counter**, which is right for a service
  and wrong for a library someone calls twice expecting independent totals.
- **Review snippets are treated as a source with no weight attached.** Two
  cherry-picked strings with no count and no distribution sit beside a contracted
  partner feed as equals. That is deliberate — nothing in this data justifies a
  trust ranking — but it is not what a production system should do.

The first thing I would do next is trust tiering: every claim already carries its
source, so attaching a tier costs little and is the honest way to help a reader
weigh a three-year-old scraped review against an undated partner feed, without the
tool picking a winner on their behalf.

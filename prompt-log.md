# Prompt log

Every instruction I gave the assistant during this exercise, in order, with what
came back. My prompts are verbatim, typos included. The responses are summarised
— the full reasoning is long, and what matters for review is what was asked, what
was pushed back on, and what got built.

The prompts, with each reply closed until you open it, are in [docs/prompts.html](docs/prompts.html). The raw dump is in [docs/session-log.html](docs/session-log.html). This file is the short read: what was asked, what was pushed back, and what got built.

The short version of how this went: the assistant was most useful when it
disagreed with me. Three of the defects it found I had missed entirely, and two
of my instructions it argued down to something better. The times I had to correct
it were about scope and ceremony, not correctness.

---

## 1 — Defect analysis, no code

> Timed 60-minute exercise, clock starts now. Read /path/to/hotels.json.
> Go only. You write all code — I write zero lines.
>
> Phase 1 (10 min, no editor): tell me every defect in this data, ranked by
> how badly it would burn a real traveller if an AI agent repeated it.
> Include anything cross-record. Do not write code yet.
>
> Then we pick ONE feature and I'll say go.
>
> my findings, you can compare with yours and let's decide what to do with them
> different filed names => name, hotel_name, description is different languages,
> location is missing in some response , rating and stars fields, amenities
> string with coma separator and features array of strings, fields can be null in
> some cases and non nulable in others

It confirmed my six findings and then said, bluntly, that all of them were schema
defects — the cheap kind, the kind a type system catches — and that none of them
would actually hurt a traveller, because a schema defect makes the tool fail
loudly. It ranked thirteen defects by harm and the top four were all ones I had
not found:

1. `City Lodge Berlin` carries Munich's coordinates (`48.1351, 11.5820`). 585km.
   Nothing inside the record contradicts itself, so no validator catches it.
2. `Sunset Bay` is "adults-only" in prose and has a "kids club" in features.
3. The two Rimini records are the same hotel with 4 stars and 3 stars.
4. The pool is advertised and a review in the same file says it was closed all
   August.

Plus currency incomparability, staleness, and the `stars` / `rating` category
error. It also pointed out that my "null vs non-null" finding was the shallow
version of a worse one: `null` being read as `false`, which is how Alpenhof's
"breakfast included" disappears.

## 2 — Pick the feature, write DECISIONS.md first

> We definitely need a normalisation and we need to fix all six harmful defects.
> Write DECISIONS.md first. Three short sections only: the feature and who it
> helps, two alternatives you dropped and why, how we'll know it works. Feature
> is a claim check. Group Mare Azzurro and Mare Azzuro into one property, then
> print each claim with its sources and a status: supported, conflict, stale, or
> absent. Don't pick a winner when they disagree.
> Build it one case at a time, and print the report after each:
> 1. Berlin city vs the Munich coordinates
> 2. adults-only vs kids club
> 3. 4 stars vs 3 stars on the Rimini pair
> 4. pool vs the August review, and keep the 2023 last_seen as stale
>
> Let me know if you have a better idea or you disagree or you think I missed smth
> and we need to add or remove anything

Three disagreements came back, and I took all three:

- **My case 1 would have silently passed.** Only one source says anything about
  City Lodge Berlin's location, so a claim check comparing sources sees no
  disagreement and reports `supported`. It needed coordinates to emit a *derived*
  claim, and claims to be keyed on `(feed, field-path)` rather than on the feed,
  so a record can contradict itself. Same for case 2 — both halves come from one
  record.
- **`stale` is not a fourth status, it is a second axis.** My case 4 asks for the
  pool conflict *and* the 2023 staleness on the same property; with one enum,
  reporting one erases the other.
- **`absent` was doing two jobs**, which is the null-means-false defect
  reappearing inside the tool built to catch it. A source saying "no restaurant"
  is `supported (false)`, not `absent`.

It also said I had missed two of the six: the Alpenhof null-vs-prose case (added
as case 5) and currency (dropped, with an argument — the two prices are on
different properties, so a claim check structurally cannot see the conflict).
And it corrected my framing: this surfaces the defects, it does not fix them.

## 3 — Go

> ok, let's focus on what we can build and test first, we can come back to the
> contradictions later and think how to resolve them.
> go, remember we have go skills go-loop and review, tdd and rules

Built end to end first, then one case at a time, failing test before
implementation each time. All five cases green.

One thing worth recording because it is the best evidence in the repo that the
tests do something: while building case 5, the tool invented a conflict. The
prose signals are substring tests, and `"Kein Restaurant"` contains
`"restaurant"`, so the positive and negative signals both fired on one
unambiguous German sentence. The test caught it, not a review. The fix is an
explicit list of phrases that contain a signal while meaning the opposite — a
patch, not a fix, and it is written up as the first known gap.

## 4 — Commit discipline

> after each phase do a commit and push so we have a clean history, not one big
> commit.

Sent mid-build. From that point the history is a `test:` commit with the failing
cases followed by a `feat:` commit that makes them pass, per case.

## 5 — Handle what we cannot predict

> One failing test per case, then the minimum code to go green, then print the
> report. Stop there. No race loop, no mutation, no linter, no extra concurrency.
>
> run a go-review, code-review skills
> after
> Let's run tests, see what's left, what's working and what's not, what other
> decisions we need to make, what to change and what to build on top, add logs,
> check for edge cases, races, code smells if we imagine its gonna be a huge
> dataset.
> also prompt-log.md isn't in the repo. add the prompts so far and commit it, or
> they won't review.
> also what I've noticed
> don't add more phrases. if a feature matches nothing, print it as unparsed with
> the raw text, so a miss isn't silence.
> same for a description no signal hit. That's about all the cases, we can fix
> what we know but think about cases which we don't know and cannot predict,
> since each api can have their own namings and set of rules, so we are fixing
> parts we are aware and correctly handling all the "unexpected".
>
> again,he five cases print. don't add more names to the lists.
> if a feature or amenity matches nothing, print the raw text as unparsed. pets
> allowed and swim-up bar are the ones in this file.
> same for a json field we have no struct field for, so a new feed can't
> disappear quietly
> tell me if you disagree or have a better idea how to handle those

This is the instruction the design actually needed, and it came from me, not from
the assistant — it had been growing phrase lists and would have kept going. The
assistant agreed on unknown JSON keys without reservation and called it the only
mechanism in the repo that catches a defect *class*. It partly disagreed on
descriptions: printing every description that matched nothing would bury the four
real conflicts under prose that was never going to map to a field, so it reports
a description only when *zero* signals fired on it and it is non-empty. I took
that.

It also flagged, and I accepted, that the unparsed facility bucket necessarily
conflates two different things — `"swim-up bar"` is a facility with no canonical
name, `"pets allowed"` is not a facility at all — and that separating them needs
exactly the vocabulary I had just told it to stop growing. So they share a bucket
and a reader decides.

Its counter-proposal for scale: the unparsed bucket should be counted and ranked
across the whole dataset, not just printed per property, so the vocabulary grows
from evidence instead of from guessed phrases.

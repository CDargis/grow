# Delivered NPK — Trend / Timeline View (Phase 2)

## Status
Deferred. Phase 1 (delivered elemental NPK grams, replacing the old percentage-based
"Total NPK") is done and deployed — see `context/decisions.md`. This is fully spec'd
so Phase 1 was built forward-compatibly, but not implemented yet.

## Background
Phase 1 computes and stores `deliveredN`/`deliveredP`/`deliveredK` (elemental grams)
on every watering/feeding log's nutrient entries. Those grams are additive across
products, feeds, and the whole grow — which is exactly what this view is for: a
per-plant feeding-trend visualization, reached from the plant detail page.

---

## Modes (toggle)

- **Rate mode (default on entry):** single plant. Stacked N/P/K bars per feeding
  (delivered grams). X-axis = calendar date (real feeding history). Tap a feed to
  expand per-product contributions. Answers "what did I do, and how did she respond."
- **Cumulative mode:** running-total lines (delivered grams accumulating over the
  grow). Comparison across plants enabled here (see below). X-axis = days-since-sprout
  (normalized — see below).

## Comparison (cumulative mode only)

- The **primary plant** (navigated from) renders fully: solid line + markers.
- **Comparison plants** render as dotted/dimmed cumulative lines only — no bars, no
  markers. Keeps it readable.
- Comparison shows **one nutrient at a time** via a nutrient selector (N, P, or K) —
  avoids 3 nutrients x N plants clutter. Usual use case is chasing one nutrient
  ("who's low on K").

## Normalized x-axis (why days-since-sprout for comparison)

Plants start at different calendar dates, so plotting cumulative lines on calendar
dates misaligns them by stage. Normalizing to days-since-sprout aligns plants by
developmental age: each plant's day-0 is its own sprout date. "At day 20, who had
received more K" becomes a fair comparison instead of an artifact of different
start dates.

- Rate mode: calendar date (single plant, real history — no normalization needed).
- Cumulative/compare mode: days-since-sprout.

## Markers on the timeline

Derive markers from existing structured log types only — do NOT parse freeform
notes for symptoms (notes are a grab-bag of weights/observations/etc.).

- **Phase changes** (flower flip, harvest, dead): full vertical lines — the
  developmental anchors.
- **Transplant, training, trimming:** smaller event ticks/icons.
- **Water/feed:** these ARE the bars (rate mode) / line inflection points
  (cumulative) — not separate markers.
- **Notes:** subtle, de-emphasized tappable dot — present for context ("what was I
  thinking on day 22") without pretending to be structured symptom data.

## Explicitly out of scope (future, not this phase either)

- A dedicated symptom/observation log type (structured, with severity) would make
  markers richer, but is a separate future feature.

---

## Build notes for when this is picked up

- Depends entirely on Phase 1's stored `deliveredN/P/K` on log entries — no new
  per-feed computation needed, just querying/aggregating what's already there.
- Needs a per-plant sprout date to compute days-since-sprout (check whether this
  is already captured via a `sprout` log type or phase-change history, or needs
  a new field).
- Chart library not yet chosen — pick one that supports stacked bars + line
  overlays + dual x-axis modes without excessive bundle size (frontend bundle is
  already flagged by Vite as approaching the 500kB chunk-size warning).

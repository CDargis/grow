# QR Share Page — public grow summary behind a label QR code

## Status
Active. Design settled (`qr-share-page.mockup.html`). Build is deliberately small.

## Goal
Printed Casimir Botanicals mylar labels carry a QR code. Scanning it opens a public,
no-login page summarizing how that plant was grown: three photos, a phase timeline,
and a nutrition summary. Provenance and story, not a product listing.

---

## Approach: a frozen template plus one JSON per grow

A handful of pages a year. No table, no API, no bucket, no migration.

```
shares/
  template.html         # the design. Frozen. Never regenerated.
  data/A7K2M9.json      # one file per grow — the only thing that changes
  photos/A7K2M9/{1,2,3}.jpg
  build.mjs             # template + json -> page, and emits the QR svg
```

`build.mjs` writes `frontend/public/p/A7K2M9/index.html` and copies the three photos
beside it. **`frontend/public/` is copied verbatim into `dist/` by Vite**, and the
existing `SiteDeployment` in `infra/src/Grow/GrowStack.cs:425` already ships `dist/`
to `grow-site`. So publishing a share page is just the normal site deploy. No new
bucket, no new CloudFront behavior, and none of the `Prune` trouble that comes from
writing into that bucket out-of-band.

It also stays a snapshot for free: a static file can't drift when the plant record is
later edited or archived, and the label keeps working forever.

### The one rule that matters

**Claude writes the JSON. Claude never opens `template.html`.**

That's the whole guarantee against the design regenerating itself. The layout, palette
and copy tone live in a file that is not an input to the per-grow task; the only
writable surface is a JSON with fixed keys. Handing over the whole page each harvest
is what produces drift — different wording, different spacing, a slightly different
green.

### What Claude does per harvest

Given a plant, using the read-only MCP connector that already exists:

1. Read the logs. Derive day 0, the phase segments, peak height, flowering time.
2. Sum `deliveredN/P/K` across every `watering` log's `nutrients[]`.
3. Compute the soil charge as plain arithmetic (below) and add it.
4. Dedupe the free-text product names by hand.
5. Write `shares/data/<code>.json`.

Then: drop three chosen photos into `shares/photos/<code>/`, run `build.mjs`, deploy.

### Photo selection

You pick them. Drop three files into `shares/photos/<code>/` named `1.jpg`, `2.jpg`,
`3.jpg` (seedling, mid flower, before harvest) and put the day number for each in the
JSON. No picker UI, no auto-suggestion — a heuristic can't tell which frame is in
focus or flattering, and these go on a physical product.

---

## The JSON

```json
{
  "code": "A7K2M9",
  "strain": "Cherry Pie",
  "batch": "Sunny",
  "harvested": "2026-08-16",
  "totalDays": 98,
  "phases": [
    { "label": "Germination", "days": 4 },
    { "label": "Seedling",    "days": 19 },
    { "label": "Vegetative",  "days": 7 },
    { "label": "Flowering",   "days": 68 }
  ],
  "photos": [
    { "file": "1.jpg", "stage": "Seedling",       "day": 12 },
    { "file": "2.jpg", "stage": "Mid flower",     "day": 57 },
    { "file": "3.jpg", "stage": "Before harvest", "day": 96 }
  ],
  "record": {
    "Seed bank": "Zamnesia",
    "Type": "Autoflower",
    "Light": "18 on / 6 off",
    "Flowering time": "68 days",
    "Tallest": "34.75 in",
    "Seed to harvest": "98 days"
  },
  "npk": { "n": 26, "p": 19, "k": 13 },
  "npkSource": "Total inputs — Ocean Forest base plus 9 hand-mixed feedings",
  "ingredients": "Built on a living-soil base and fed by hand: ..."
}
```

`record` is an ordered object rendered as-is, so adding or dropping a row never
touches the template.

---

## Deriving the numbers

### Day 0 and the phase segments

From `phase_change` logs, replayed in `logId` order (ULID = chronological). Day 0 is
the transition into `germination`, so "seed to harvest" counts what the phrase says.

**`plant.createdAt` is not a fallback.** It's a data-entry timestamp. Sunny, Cherry and
Sid were created 68 seconds apart (`20:42:11` / `20:42:58` / `20:43:19` on 2026-05-17),
a full week after Sunny's germination began on 2026-05-10 — using it gave 91 days
against a true 98. If a plant has no `germination` transition, ask for the date. Never
guess: a wrong day count on a printed label is permanent.

Sunny's real arc: germination 2026-05-10, seedling 05-14, veg 06-02, flower 06-09,
harvest 08-16 → 4 / 19 / 7 / 68, total 98 days.

Post-harvest phases (`drying`, `curing`) stay off the bar. The page is published after
cure, so showing them as live state says nothing.

### Nutrition

Grams of elemental N, P, K — a weight, never the label-style `5-5-10` concentration
triple. A percentage describes a bottle; grams describe what the plant received, and
only grams are additive across soil and feeds.

**Feed:** sum `NutrientEntry.DeliveredN/P/K` over every `watering` log. Already stored
per entry (`decisions.md`, 2026-07-30). Nil means unknown — sum what exists.

**Soil:** arithmetic, not a data model. From the bag:

```
bulk density = bag lb / bag cu ft
mass (g)     = gallons x 0.13368 x density x 453.59
N = mass x N%           P = mass x P2O5% x 0.436          K = mass x K2O% x 0.83
```

The 0.436 / 0.83 factors match `model.ElementalFromLabel` — same conversion, done by
hand rather than modeled.

Worked for Sunny — FFOF, 0.30-0.45-0.05, 1.5 cu ft at 34 lb, 5 gal:

| grams | N | P | K |
|---|---|---|---|
| Ocean Forest, 5 gal (6,872 g) | 20.6 | 13.5 | 2.9 |
| 9 feedings | 5.2 | 5.7 | 10.2 |
| **Total inputs** | **26** | **19** | **13** |
| soil's share | 80% | 70% | 22% |

**The soil is not optional.** Feed-only reads K-dominant (10.2 K vs 5.2 N), a
bloom-heavy profile. Total inputs reads N-dominant. Opposite stories, and the
feed-only one is wrong.

**Whole grams only.** `potSizeGal` is the container, not the media volume — fabric pots
aren't filled to the brim, and Sunny spent 20 days in a solo cup before the 05-30
transplant. Comfortably ±10%; a decimal claims accuracy that isn't there.

Say **"total inputs"**, not "delivered" — soil NPK is present and available, not
necessarily taken up.

---

## Page content

1. **Three photos**, hand-picked, each stamped `Stage · Day N`.
2. **Phase timeline** — the real phases named plainly. No invented umbrella labels: an
   early draft said "Growing · 23 days", which is vague *and* wrong, since the plant
   grows for all 98 of them.
3. **Nutrition** in elemental grams, soil included.
4. **The record** — seed bank, type, light, flowering time, tallest, seed to harvest.
   Not container size; it says nothing to a reader.
5. **Ingredients statement** — hand-written, foregrounding the organic inputs. An
   ingredients panel, not a diary entry.

Plain language over an explainer paragraph: "eighteen hours of light" in prose,
`18 on / 6 off` in the table. Nobody reads an explainer on a page they scanned at a
party.

**Off the page:** real name, city, the room, other plants, any link into the app, and
any potency figure without a lab result behind it.

**Claims must survive scrutiny.** The ingredients copy leads on organic, and most
inputs support it — fish emulsion, worm castings and tea, Dr. Earth, langbeinite. But
Cultivation Nation Bloom and CalMag are mineral, and they're in most feeds, so the copy
names both sides plainly rather than claiming "organically grown" outright. "No
synthetic pesticides at any point" was confirmed with the grower (2026-08-17).

---

## Visual identity: Casimir Botanicals

Source of truth: `~/Downloads/casimir-botanicals-labels.html`.

| Token | Value | Use |
|---|---|---|
| `--ink` | `#1E3126` | body text |
| `--paper` | `#E9E0CA` | ground |
| `--oxblood` | `#8A3324` | eyebrows, NPK figures |
| `--brass` | `#A6802B` | dividers, flowering segment |
| `--sage` | `#8B9A7C` | growth segments |

Type: **Pinyon Script** wordmark, **Cinzel** small caps, **Cormorant Garamond** prose,
**IBM Plex Mono** on every measured value. The mono is load-bearing — this is a record
of measurements and should read as one.

**Single-theme by intent.** The label is cream paper, so the page is cream paper.
Packaging doesn't theme-switch.

**Self-host the fonts.** All four are Google Fonts; the page must not `<link>`
fonts.googleapis.com and silently fall back to Georgia. Latin subsets are ~115 KB
across five faces (Pinyon 39, Cormorant roman+italic 46, Cinzel 15, Plex Mono 15).
Consider making the wordmark an SVG and dropping Pinyon.

---

## Label / QR practicalities

- **Matte label stock.** Gloss over shiny mylar fails to scan under overhead light.
  This is the number-one real-world failure.
- **Print ≥1.0" square** with a full quiet zone. `https://grow.chrisdargis.com/p/A7K2M9`
  is 37 characters → QR Version 3 (29×29) in byte mode at EC M. At 1.0" that's 0.034"
  per module, well clear of where phone cameras struggle. No second domain needed.
- **EC level M**, or Q if a logo goes in the center.
- **Print the URL as text under the code.** QRs fail; text doesn't.
- **Dark on light.** Inverted QRs are a coin flip across scanner apps.
- **One code per harvest**, not per bag.
- Codes are short opaque strings, not `plantId` — a ULID would leak creation time and
  make neighbors guessable.

---

## Deliberately not built

Dropped as overkill for a few pages a year: a `grow-shares` DynamoDB table; `POST`/
`DELETE /api/shares`; a separate S3 bucket with its own CloudFront behavior; a Go
`html/template` renderer with image resizing and EXIF stripping; a publish UI with a
photo picker and shares list; a `ProductMedium` kind on `Product` with `TransplantData`
soil snapshots and a `migrate-soil-npk` backfill.

If shares ever become frequent enough that hand-running a script is the bottleneck,
that's when to revisit — not before.

**One small loose end:** a mistyped code falls through CloudFront's 403/404 rewrite
(`GrowStack.cs:384`) into the SPA, and `AuthProvider.tsx:47` then redirects a stranger
to the Cognito login. A public-route early return for `/p/*` fixes it in ~5 lines.
Only reachable by typing a bad code by hand, so it's a nicety, not a blocker.

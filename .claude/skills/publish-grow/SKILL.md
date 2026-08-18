---
name: publish-grow
description: Build the data file for a Casimir Botanicals grow-record share page (the page behind the QR code on a mylar label). Reads a finished plant's logs via the Grow MCP connector, derives the phase timeline and total NPK inputs, and writes shares/data/<code>.json. Use when asked to publish, share, or make a grow page / label page / QR page for a plant.
---

# Publish a grow record

Produce `shares/data/<code>.json` for one finished plant. Nothing else.

## The one hard rule

**Never open, edit, or regenerate `shares/template.html`.**

That file is the design — layout, palette, copy tone, the Casimir Botanicals
identity. It is not an input to this task and not something to "improve while
you're in there". The only file you write is the JSON. If the page looks wrong,
say so and stop; don't fix it by rewriting the template.

Likewise, phase colours live in `build.mjs`, not the data. A data file must never
be able to restyle the page.

## Steps

1. **Find the plant.** `list_plants`, match the name given. It must be past
   harvest (`harvest`, `drying`, `curing`, or `archived`). If it isn't, stop and
   say so — you cannot publish a plant that's still growing.

2. **Pick a code** if one doesn't exist. 6 characters, Crockford base32
   (`0-9 A-Z`, no `I L O U`). Check `shares/data/` for collisions. One code per
   harvest, not per bag.

3. **Read the logs.** `list_logs_for_plant` with a high limit. It caps out and
   spills to a file — parse that file rather than assuming you got everything.
   **Check the earliest date you actually received against the plant's history.**
   A truncated window silently loses the early phase changes, which is exactly
   where day 0 lives.

4. **Derive the timeline** from `phase_change` logs in `logId` order (ULID =
   chronological).
   - **Day 0 is the transition into `germination`**, so "seed to harvest" counts
     what the phrase says.
   - **`plant.createdAt` is NOT a fallback.** It's a data-entry timestamp. Three
     of Chris's plants were created 68 seconds apart, a week after germination
     actually began — using it gave 91 days against a true 98. If there's no
     `germination` transition, **ask for the date**. Never guess: a wrong day
     count on a printed label is permanent.
   - Post-harvest phases (`drying`, `curing`) stay off the timeline. The page is
     published after cure; showing them as live state says nothing.
   - Phase labels must be the real phases (`Germination`, `Seedling`,
     `Vegetative`, `Flowering`). Never invent friendly umbrella labels — an early
     draft said "Growing · 23 days", which is vague *and* wrong, since the plant
     grows for all 98 of them.
   - `phases[].days` must sum exactly to `totalDays`. The build enforces this.

5. **Total NPK inputs**, in whole grams of elemental N/P/K.
   - **Feed:** sum `deliveredN/P/K` across every `watering` log's `nutrients[]`.
     Nil means unknown — sum what exists and mention the gap.
   - **Soil:** look up the transplant log's `medium` string in
     `shares/media.json` and compute:
     ```
     mass_g = gallons * 0.13368 * (bag_lb / bag_cuft) * 453.59
     N = mass_g * N%            P = mass_g * P2O5% * 0.436
     K = mass_g * K2O% * 0.83
     ```
     If the medium isn't in `media.json`, **ask for the bag's guaranteed
     analysis, weight and cubic feet** and add the entry. Don't guess from
     memory — these numbers go on a permanent label.
   - **The soil is not optional.** For Sunny it was 80% of total nitrogen.
     Feed-only reads K-dominant; total inputs reads N-dominant. Opposite stories,
     and the feed-only one is wrong.
   - **Whole grams only.** `potSizeGal` is the container, not the media volume —
     pots aren't filled to the brim, and a plant's early days are in a smaller
     one. It's an upper bound, ±10%. A decimal claims accuracy that isn't there.
   - Say **"total inputs"**, never "delivered" — soil NPK is present and
     available, not necessarily taken up.

6. **Write the ingredients statement.** It's an ingredients panel, not a diary
   entry and not the grower's personal voice. Lead on the organic inputs.
   **Claims must survive scrutiny:** name mineral/synthetic products plainly
   rather than claiming "organically grown" outright when they're in the feeds.
   Never assert anything the logs don't support — pest/spray claims, unattributed potency, or
   lab results. Ask before writing one.

7. **Dedupe product names by hand.** Free-typed nutrient names arrive as
   variants — `CN Bloom`, `CN bloom`, `Cultivation Nation Bloom`, and
   `Worm castings ` with a trailing space. Nothing derives this; collapse them
   and use the full proper name.

8. **Fill `record`** — an ordered object, rendered as-is. Seed bank, type, light
   schedule, flowering time, tallest, seed to harvest. **Not container size**; it
   says nothing to a reader.

9. **Write the JSON**, including a `_derivation` block showing where day 0, the
   feed sum and the soil charge came from, so the numbers can be re-checked later.

10. **Stop and report.** Tell Chris:
    - the three photos you need, **by stage and day number**, and that they go in
      `shares/photos/<code>/` as `1.jpg`, `2.jpg`, `3.jpg`
    - that the `photos[].day` values in the JSON are placeholders until he picks
      the actual frames, and he should tell you the real day numbers
    - the build command: `node shares/build.mjs <code>`

## Potency

Optional. Omit the whole `potency` block if there are no figures, and set an
individual value to `null` to hide just that one -- **never print a 0% or a
blank tile.** Cherry Pie Auto shows THC and no CBD; the template renders one
tile or two accordingly.

**Always say where the number came from**, in `potency.note`:
- a strain's advertised figure -> "Breeder figure for the strain — not a lab
  test of this harvest"
- an actual assay -> name the lab and the date

A bare percentage on a public page reads as a measurement of what is in the
bag. Attributing a breeder figure costs one line and keeps the claim true.
Never carry a number over from another batch, and never estimate one.

## Photo selection is not yours

Chris picks the three frames. Do not choose them, and do not auto-suggest by date
or phase. A heuristic can't tell which frame is in focus, well lit, or
flattering, and these go on a physical product. Your job is only to say which
three *stages* are needed and to record the day numbers he gives you.

## Never on the page

Real name, the room, other plants, any link into the app. The brand shows its
town (Lisle, Illinois) on both label and page -- that is deliberate, but the
street-level detail is not. The page is a grow journal, not a product listing.

## Reference

Full spec and reasoning: `context/plans/active/qr-share-page.md`.

# Oils — Lots, Batches, and the ROBP

## Status
Active. Design settled 2026-11-07. All five steps built and deployed (2026-11-07): `internal/model/oil.go` + `internal/oilcalc` (math), `internal/store/oils.go` + `internal/oils` (rules, REST `/api/oils/*`, CDK tables), `internal/mcpserver/oils.go` (MCP tools), `frontend/src/pages/Oils` (`/oils` list + `/oils/:batch` ROBP, read-only). First real use: 3 batches from 2 lots logged via the Claude app.

## What it is
Track the oils Chris makes at home: Red Mesa isolate/distillate in MCT, THC oil from
dispensary flower, full-spectrum CBD oil from Rogue Origin flower, and eventually oils
from his own homegrow plants. **The main point is capturing COAs**, so every oil has a
record of how it was made and what's in it.

**Simplicity is the requirement.** The ideal workflow is Chris in the kitchen telling
Claude (claude.ai app) what he's doing, and Claude writing it all into grow via MCP.
No data-entry forms to start.

## Why it lives in grow
Considered a separate app (the concern was domain leakage: grow is for *growing*).
Kept it in grow because:
- Auth, Cognito, MCP connector, pipeline, and frontend already exist. A separate app
  would rebuild all of it.
- Homegrow flower → oil is a real link. A flower input can point at a `plantId`, giving
  seed → plant → harvest → oil.

Leakage is contained: own tables, own MCP tools, own top-level route. The only link to
the growing side is the optional `plantId` on a flower input.

---

## Concepts
- **Lot**: always the *supplier's* lot. One purchased jar of starting material
  (e.g. a Red Mesa isolate jar), identified by the supplier's lot number, covered by
  the supplier's COA. Lots in, batches out.
- **Batch**: something Chris made. Its inputs are lots, earlier batches, flower, and
  additives (terps etc.).
- **ROBP (Record of Batch Profile)**: the calculated profile of a batch. Our own
  "COA", but calculated, not lab-tested. Not a certificate; nothing official.

Out of scope: containers/bottles, grams remaining in a lot, a dose log. Dose math is a
stateless tool call, not stored.

## Profile (shared by lots and batches)
One standard storage format for everything a COA reports. Claude extracts the values
from the COA (layouts vary too much to parse programmatically) and fills the format.

- **Cannabinoids**: fixed known list (CBD, CBDA, Δ9-THC, THCA, Δ8-THC, CBG, CBN, CBC,
  CBDV, THCV, …), mg/g.
- **Terpenes**: fixed known list (myrcene, limonene, β-caryophyllene, linalool, …), mg/g.
- **Other**: name/value pairs for anything the lab reports that isn't on either list,
  so nothing gets thrown away.

Lots get their profile from the COA. Batches get theirs calculated from inputs.

## Lot
- Supplier, product (isolate / distillate / …), supplier lot number, grams purchased
- Red Mesa today, other suppliers later. Supplier lot numbers can collide across
  suppliers, so uniqueness is on (supplier, supplier lot number). `lotId` is an
  internal ULID.
- COA URL + profile extracted from the COA
- COA archive: when a lot is saved, the Lambda does a plain HTTP GET of the COA URL and
  stores a copy in S3, so a lab moving or deleting the file doesn't lose the record.
  If the fetch fails, the lot still saves and is flagged "not archived."
  (MCP never moves files. Chris gives Claude the PDF to read and the link to store.)

## Batch
- **ID**: built by the app from its parts, never parsed:
  `[type]-<subType>-<discriminator>-[YYMMDD]-<batchDiscriminator>`
  (`[]` required, `<>` optional). Parts are stored as separate fields; the app builds
  the string and checks it's unique. `-2`-style batch discriminator only when needed.
  - Types: `DIS`, `ISO`, `FS`, `THC`, `BL`
  - Unknown date: date field is null, ID uses sentinel `000000`
  - Examples: `DIS-261024`, `ISO-261024`, `FS-CBD-BB-260717` (CBD plant, Berry Bliss),
    `THC-TST-000000` (small test, blended flower, date unknown)
- Date made, total grams made, carrier oil (brand/product) + grams, carrier density
  (g/mL, per batch, default 0.95 for MCT), notes
- **Inputs** (array, grams each), each one of:
  - a lot (raw ISO/DIS straight from the jar)
  - an earlier batch (e.g. a previously made ISO oil)
  - flower: grams of flower, strain, the label/COA profile (as mg/g; 1% = 10 mg/g),
    optional `plantId`, **method** (free text), **confidence** (low / medium / high,
    set by Chris), and an **efficiency** factor (0–1) that Chris and Claude settle on
    per batch and pass in. The app applies decarb (acid × 0.877) and the efficiency to
    get the flower's contribution to the oil.
  - an additive (terps etc.) with grams. The API only takes grams; Claude converts
    drops (or whatever Chris says) to grams before calling, and the original wording
    can go in the input's note.
- Optional batch-level lab COA (expensive, but especially wanted for homegrow flower
  oils). When present it's shown next to the ROBP, showing how far the estimate was off.

## Calculated numbers
- **Units**: mg/g is the single stored source of truth (it's what Chris is making).
  mg/mL is derived as mg/g × density (it's how Chris doses). Density is stored per
  batch, default 0.95 g/mL for MCT. MCT oils all sit around 0.93–0.96, so the default
  is off by ~2% at most, which is fine for dosing. It's stored so it can be corrected
  if it's ever measured (weigh 10 mL). Everything displays both units.
  Known carriers: LouAna Liquid Coconut Oil ≈ 0.94 (80–85% C8/C10 MCT per LouAna;
  calculated from composition, not measured). Pure MCT ≈ 0.95.
- **Batch profile** = weighted blend of inputs by grams, per compound.
- **Estimated vs lab, per compound**: for each cannabinoid (and terpene), the share of
  that compound's mg that came from flower guesses vs lab-backed inputs, weighted by
  **contribution, not grams**. Estimated portions are broken down by confidence label.
  Display: `CBD 24.8 mg/g · 60% medium · 40% lab`.
  - Per compound because sources differ: an ISO + THC-flower blend is ~0% estimated
    for CBD but 100% estimated for THC. Chris makes some batches for CBD, some for THC.
  - Lab-backed includes ISO/DIS batches (COA numbers diluted by carrier, which is just
    arithmetic).
  - Estimates and labels carry through into any batch made from an estimated batch.
- **THC level** is calculated from the profile, never a typed-in "THC-free" flag
  (distillate often has trace THC). Any "THC-free" display is derived from the number.
- CBD is listed first everywhere; THC alongside.

## Calculation rules (as built in oilcalc)
- **Decarb** (acid × 0.877 → THC/CBD, CBGA × 0.878 → CBG) applies to flower inputs
  only. Lot and lab COAs are taken as reported: an oil that lists THCA has THCA.
- Flower terpenes are scaled by the same efficiency factor (a guess either way).
- **Additives** count as lab-backed (known composition, e.g. pure limonene).
- **Not detected** is stored as 0 (present key), distinct from "not reported" (absent).
  A 0-mg compound keeps a provenance split by grams of the inputs that reported it, so
  "THC 0 · 100% lab" still shows for an isolate oil.
- **Totals**: Total CBD = CBD + 0.877 × CBDA; Total THC = Δ9-THC + 0.877 × THCA. Δ8-THC
  is its own line.
- A batch's lab COA replaces its calculated profile, including downstream (as 100%
  lab); the calculated profile is still returned for comparison.
- Profile values are validated 0–1000 mg/g; unknown keys must go in `other`.
- Batch code parts must be letters and digits (a hyphen inside a part would make the
  code ambiguous) and are uppercased.

Worked example (100 g blend):

| Input | Grams | CBD mg/g | CBD mg |
|---|---|---|---|
| FS (medium) | 99 | 15 (estimate) | 1,485 |
| ISO lot | 1 | 990 (lab) | 990 |
| **Batch** | 100 | **24.8** | 2,475 |

→ `CBD 24.8 mg/g · 60% medium · 40% lab`

## MCP tools
Built. The grow MCP server was read-only before this (list_plants, get_plant,
list_logs_for_plant, get_logs_by_date_range). This feature adds its first write tools.
- `create_lot`: supplier details, COA URL, extracted profile
- `create_batch`: ID parts, inputs, carrier, totals, notes; responds with the
  calculated ROBP (mg/g + mg/mL + estimated/lab split) for Claude to read back
- `update_lot`, `update_batch`: fix or fill in fields after the fact (COA numbers,
  grams, notes, ID parts, inputs)
- `list_lots`, `list_batches`, `get_batch`
- `delete_lot`, `delete_batch`: remove a mistaken record; refused while something depends
  on it (added after first use)
- `dose`: "how much oil for 100 mg CBD from batch X" (stateless math, mL + g)

## UI
New top-level route next to plants / environments / overview / inventory. Read-only
to start (entry happens through MCP).
- `/oils`: batch list (ID, type, date, CBD/THC mg/g and mg/mL, estimated share);
  lots as a second tab/section with COA link + archived status. Lots stay out of
  `/inventory` (that's grow supplies).
- `/oils/:batchId`: the ROBP. Inputs with grams, full profile with mg/g, mg/mL and
  estimated/lab split per compound, method + confidence, links to source COAs, source
  batches, and plants. "Calculated, not lab-tested" note. Batch lab COA alongside if
  present.

## Storage
- `grow-oil-lots` (PK `lotId` ULID, GSI `user-index`) and `grow-oil-batches` (PK
  `batchId` ULID, GSI `user-index`). Interfaces in `context/schema.md`.
- No `user-code-index` GSI after all: uniqueness of batch codes and of (supplier, lot
  number) is checked in Go against the user's full list, which is already loaded to
  calculate profiles. Lookup by code works the same way.
- `code` (the human batch ID) is a unique label, not the key, so changing ID parts on
  update doesn't break blends that reference the batch.
- **Profiles are calculated on read, never stored.** Updates to a batch flow into every
  blend made from it automatically. Volume is tens of items, so list loads everything
  and computes in memory. If that ever gets slow, move to a background Lambda that
  recomputes and stores profiles after each write.
- `update_batch` rejects input cycles (a batch can't contain itself, directly or
  through other batches) and returns the blends whose numbers changed.

## Flower sources (2026-11-07)
Full-spectrum and THC oils come from three kinds of flower, and they differ mainly in how
good the *starting* numbers are:

| Source | Starting numbers | COA? |
|---|---|---|
| Bought flower/hemp with a COA (e.g. Rogue Origin Berry Bliss) | the flower's COA | yes, often a JPG |
| Bought flower with only a label % (dispensary, some online hemp) | label | no |
| Homegrown (`plantId` set) | breeder/seed bank's stated range, or a guess | rarely; seeds don't come with COAs |

Built: a flower input can carry `coaUrl`; the app archives its own copy (image or PDF,
content type sniffed when the host sends octet-stream) under `oils/batches/{id}/flower-coa/`,
keeps it when the same link is re-sent, and removes it when the link changes or the batch is
deleted. The flower's numbers still count as estimates: that COA documents the flower, not the
oil.

Open: `confidence` currently has to cover two different things: how much Chris trusts the
**method** (extraction) and how good the **starting numbers** are (a COA vs a label vs a
breeder's "15–20% CBD"). Proposed: add `profileSource: 'coa' | 'label' | 'breeder' | 'guess'`
to flower inputs, shown on the ROBP, informational only (Chris still sets one confidence,
weighing both). Homegrow path to better numbers: send a flower sample to a lab, then
`update_batch` with the new profile + `coaUrl`; blends made from it update automatically.

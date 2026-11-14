# Dose — Plans, Versions, and the Shelf

## Status
Design settled and built 2026-11-14: `internal/model/dose.go`, `internal/dose` (rules +
`Derive`), `internal/store/dose.go`, `mcpserver/dose.go`, `GET /api/dose`, tables
`grow-dose-versions` / `grow-shelf`, and the Oils → Dose segment (`pages/Oils/Dose.tsx`).
Builds on oils (`context/plans/active/oils.md`). Next: Chris enters the current plan via
the Claude app. Canonical schema is in `context/schema.md`; the draft schema below used
earlier names (slots → times, ShelfSlot → ShelfItem).

## What it is
Chris's daily dose regimen: mostly his own oils, sometimes a gummy or a premade oil.
The regimen changes every 2–6 weeks. **It's a plan, not a log.** No daily "what I
took" logging; that's too much work. Replaces a hand-built dose card (web Claude
artifact) whose oil strengths were typed in by hand.

Same workflow as oils: Chris tells Claude (app) what's changing, and Claude writes it
via MCP. The app view is read-only.

## Where it lives
The third segment of the Oils page: **Batches · Lots · Dose**. No sixth bottom-nav
tab. It belongs with oils because the strengths come from batch ROBPs. A separate
home would have to copy them, and they'd go stale.

---

## Concepts

Two independent things change, so they're two layers:

### 1. Plan: *how much* (changes when the dose changes)
Time-of-day slots (Morning / Midday / Night), each with lines. A line is
`{ shelf slot, mg, compound }`, e.g. "260 mg CBD from *Distillate oil*". It names a
shelf slot, **never a batch directly**.

### 2. Shelf: *which bottle* (changes when a bottle changes)
Named slots → current source:
```
Distillate oil  →  batch DIS-260712       (mg/mL from its ROBP)
Isolate oil     →  batch ISO-260611
THC oil         →  batch THC-0000-TST     (low confidence, shown as a chip)
Peach Power     →  product: gummy, 40 mg CBD each
```
- Opening a new batch with the same dose: repoint the slot. The plan doesn't change,
  and the mL recalculates.
- Shuffling around what's available: repoint a slot or tweak a plan line.
- **Products** (premade oils, gummies): label strength (mg/mL or mg/unit), optional
  COA link (archived like lot COAs). No ROBP behind them.
- Each slot keeps a dated source history, so "what was I taking in March" is answerable.

### Versions
A plan is a sequence of versions. Each version has `startDate` and an optional
`endDate`; the version with no end date is current.
- **Version bump**: ends the current version, and the new one starts. For a dose change.
- **Amend**: edits the current version in place, with no history. For typos and small tweaks.
- **Chris decides.** The update tool requires an explicit `mode: 'bump' | 'amend'`.
  The MCP instructions say: if Chris didn't say which, ask, don't guess.

### Drafts
A draft is just **the next version with a future (or empty) start date**. There's no
separate draft entity. The usual gap from draft to active is about 24 hours.
1. "Draft a distillate-led plan, same CBD per time of day" creates a pending version.
2. The app shows **Current | Draft** side by side: per-slot mL, day totals, minors,
   and monthly use.
3. "Start it" / "start it Monday" sets the start date, which ends the current version.
At most one pending version at a time (YAGNI; revisit if it's ever needed).

## Derived on read (never stored, same as ROBPs)
Per line: mL = mg ÷ the current source's mg/mL, using the ROBP for batches and the
label for products. Then:
- day totals per compound, including minors from the ROBPs
- mL per day and per 30 days for each shelf slot
- raw starting material per 30 days (g of each lot), traced through batch inputs
  by `oilcalc`
- confidence chips carried over from the ROBP

## Schema (draft)
```ts
interface ShelfSlot {
  slotId: string; userId: string;
  name: string;                       // "Distillate oil"; unique per user
  source: ShelfSource;                // current
  history: Array<{ source: ShelfSource; from: string /* YYYY-MM-DD */ }>;
}
type ShelfSource =
  | { kind: 'batch'; batchId: string }
  | { kind: 'product'; name: string; unit: 'mL' | 'unit';
      mgPerUnit: Partial<Record<CannabinoidKey, number>>;   // per mL or per piece
      coaUrl?: string; coaS3Key?: string };

interface DosePlanVersion {
  versionId: string; userId: string;
  startDate?: string;                 // absent = draft with no date yet
  endDate?: string;                   // absent = current (or pending)
  name?: string;                      // "Distillate-led"
  slots: Array<{ name: string;        // Morning / Midday / Night
    lines: Array<{ shelfSlotId: string; mg: number; compound: CannabinoidKey }> }>;
  notes?: string;
  createdAt: string; updatedAt?: string;
}
```

## MCP tools (draft)
`get_dose` (current + pending, derived), `list_dose_history`, `create_dose_version`
(pending/draft), `update_dose_version` (`mode: bump | amend`), `start_dose_version`,
`delete_dose_version` (drafts only), `list_shelf`, `set_shelf_slot` (create or repoint;
appends history), `delete_shelf_slot` (refused while a plan line uses it).

## Settled details
- Night is currently half a Sunset Lake Vibe gummy (30 mg CBD each, discontinued): a
  shelf slot holding a product. The plan line is 15 mg CBD and the app shows "½ gummy". When the
  gummies run out, the slot is repointed to an oil and the plan line is unchanged.
- History: a list of full past versions first. A diff between versions ("Midday 350 → 310")
  can be added later with no data changes, since versions are stored in full.

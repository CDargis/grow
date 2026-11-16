package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cdargis/grow/internal/dose"
)

// Dose tools are thin wrappers over internal/dose, which owns the version
// and shelf rules.

const doseInstructions = `

Dose plan: Chris's daily regimen (mostly his oils, sometimes a gummy or premade oil). It's a
plan, not a log: never record what he took on a given day.

- Two layers. The plan (times of day → lines of mg CBD/THC) says how much. The shelf says which
  bottle fills each named item ("Distillate oil" → batch DIS-260712). Plan lines name a shelf
  item, never a batch.
- New bottle, same dose → set_shelf_item to repoint the item. Don't touch the plan.
- Dose change → update_dose_version. Every update needs mode: bump (new version, the old one goes
  to history) or amend (edit in place, no history). If Chris didn't say which, ask him. Don't guess.
- Drafts: create_dose_version without a startDate makes a draft he can compare with the current
  plan in the app. start_dose_version makes it live (today, or a date he gives).
- mg in a plan line is total CBD or total THC (acid ×0.877 included). The app works out mL or
  pieces from the shelf item's current strength. Never send mL.
- Premade products and gummies: set_shelf_item with a product source and the label strength
  (mgPerUnit per mL or per piece). mgPerUnit takes terpene keys too, so the day total shows
  terpenes. Terpene COAs usually report % or mg/g: convert to mg per unit with the unit's weight
  (a gummy's weight from the COA or label; for an oil, mg/g × density, about 0.92-0.95 g/mL).
  If the weight isn't given anywhere, ask Chris. Don't guess.
- A product can have several COAs (e.g. one for cannabinoids, one for terpenes): send them all in
  coas, each with a label. Updating a product means sending its whole source again, including
  every COA, because what you send replaces what's there. Read it first with list_shelf.
- After a change, read back each time of day with the mg and the resulting mL/pieces, plus the
  day's total CBD and THC. Mention any line error.`

func (d Deps) addDoseTools(s *mcp.Server, userID string) {
	if d.Dose == nil {
		return
	}
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := &mcp.ToolAnnotations{DestructiveHint: ptr(true)}

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_dose_plan",
		Description: "The dose card: the current version and the draft (if any), each with every time of day's " +
			"lines worked out to mL or pieces from the shelf's current strengths, day totals (CBD, THC, minors, terpenes), " +
			"use per 30 days per shelf item, and raw lot grams per 30 days. Plus past versions and the shelf.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return nil0(d.Dose.Get(ctx, userID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_shelf",
		Description: "Every shelf item (named role → current batch or product), its strength per mL or piece, and its source history.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		items, err := d.Dose.ListShelf(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"shelf": items}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "set_shelf_item",
		Description: "Create a shelf item, or repoint one to a new bottle (the old source goes to history and the " +
			"plan's mg stays the same), or rename it. Source is a batch (code or batchId) or a premade product " +
			"with its label strength.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args dose.SetShelfItemRequest) (*mcp.CallToolResult, any, error) {
		return nil0(d.Dose.SetShelfItem(ctx, userID, args))
	})

	type deleteShelfArgs struct {
		Name string `json:"name" jsonschema:"shelf item name or shelfItemId"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "delete_shelf_item",
		Description: "Permanently delete a shelf item added by mistake. Refused while any plan version, past ones " +
			"included, uses it. Repoint it instead when the bottle changes. Confirm with Chris before calling.",
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteShelfArgs) (*mcp.CallToolResult, any, error) {
		if err := d.Dose.DeleteShelfItem(ctx, userID, args.Name); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted": args.Name}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_dose_version",
		Description: "Create a plan version. With no plan yet it becomes the current plan. Otherwise, without a " +
			"startDate it's a draft to compare in the app; with one it's scheduled and the current plan ends that " +
			"day. Only one draft at a time. Shelf items must exist first (set_shelf_item).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args dose.CreateVersionRequest) (*mcp.CallToolResult, any, error) {
		return nil0(d.Dose.CreateVersion(ctx, userID, args))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "update_dose_version",
		Description: "Change the plan. mode is required: bump = a dose change, ends the current version and starts " +
			"a new one (history kept); amend = edit in place with no history (typos, tweaks, or editing the draft). " +
			"Ask Chris which if he didn't say. times replaces the whole plan.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args dose.UpdateVersionRequest) (*mcp.CallToolResult, any, error) {
		return nil0(d.Dose.UpdateVersion(ctx, userID, args))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "start_dose_version",
		Description: "Make the draft live: today, or on a later date (the current plan ends that day). Calling it again on a scheduled draft moves its date.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args dose.StartVersionRequest) (*mcp.CallToolResult, any, error) {
		return nil0(d.Dose.StartVersion(ctx, userID, args))
	})

	type deleteVersionArgs struct {
		VersionID string `json:"versionId,omitempty" jsonschema:"omit for the draft"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "delete_dose_version",
		Description: "Discard the draft. Started versions are history and can't be deleted. If the draft was " +
			"scheduled, the current plan carries on. Confirm with Chris before calling.",
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteVersionArgs) (*mcp.CallToolResult, any, error) {
		if err := d.Dose.DeleteVersion(ctx, userID, args.VersionID); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"deleted": "draft"}, nil
	})
}

package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cdargis/grow/internal/oilcalc"
	"github.com/cdargis/grow/internal/oils"
)

// Oils tools are thin wrappers over internal/oils, which owns every rule
// (validation, uniqueness, ownership, cycle checks, COA archiving). Errors
// from the service are returned as tool errors, so Claude sees the reason
// ("batch code ISO-261024 is already used ...") and can fix the call.

// oilsInstructions is sent to the client as server instructions. It carries
// the workflow and conventions that don't fit in a single tool description.
const oilsInstructions = `Oils: Chris makes oils at home and logs them by talking to Claude, often while
in the kitchen. Lots are what he bought (a supplier's jar with its COA). Batches are what he made.

- Units: every potency number is mg/g. Convert percentages (1% = 10 mg/g). mg/mL is
  calculated by the app from mg/g and density, so never send mg/mL.
- COAs: read every value off the COA, not just CBD and THC. "ND" or "<LOQ" is 0. Values that
  have no key go in profile.other. Pass the COA's public link as coaUrl.
- Before create_lot, check list_lots for the same supplier and lot number and reuse it.
- Batch codes are built by the app: [type]-<subType>-<discriminator>-[YYMMDD]-<batchDiscriminator>.
  Send the parts, not the code. Unknown date → omit it (code gets 000000).
- Flower inputs need efficiency (0-1) and confidence (low/medium/high). Agree both with Chris
  rather than guessing. Send flower label numbers as reported (THCA/CBDA); the app decarbs.
- Additives such as terpenes: convert drops to grams before sending; put the original
  wording ("3 drops") in the input's note.
- After a create or update, read back the result in a sentence or two: the code, total CBD and
  total THC in mg/g and mg/mL, and how much of each is estimated vs lab. Mention any
  archiveError and any affected batches.`

// batchSummary is the compact list_batches row: enough to pick a batch and
// see its headline numbers without the full profile.
type batchSummary struct {
	BatchID       string            `json:"batchId"`
	Code          string            `json:"code"`
	Type          string            `json:"type"`
	Date          string            `json:"date,omitempty"`
	TotalGrams    float64           `json:"totalGrams"`
	DensityGPerMl float64           `json:"densityGPerMl"`
	FromLabCOA    bool              `json:"fromLabCoa,omitempty"`
	TotalCBD      *oilcalc.Compound `json:"totalCbd,omitempty"`
	TotalTHC      *oilcalc.Compound `json:"totalThc,omitempty"`
	ProfileError  string            `json:"profileError,omitempty"`
}

func summarize(v oils.BatchView) batchSummary {
	s := batchSummary{
		BatchID: v.Batch.BatchID, Code: v.Batch.Code, Type: string(v.Batch.Type), Date: v.Batch.Date,
		TotalGrams: v.Batch.TotalGrams, DensityGPerMl: v.Batch.DensityGPerMl, ProfileError: v.ProfileError,
	}
	if v.Profile != nil {
		s.FromLabCOA = v.Profile.FromLabCOA
		s.TotalCBD = &v.Profile.TotalCBD
		s.TotalTHC = &v.Profile.TotalTHC
	}
	return s
}

func (d Deps) addOilTools(s *mcp.Server, userID string) {
	if d.Oils == nil {
		return
	}
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	// ── Lots ─────────────────────────────────────────────────────────────────

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_lots",
		Description: "List every supplier lot (bought isolate, distillate, etc.) with its COA numbers in mg/g and whether the COA file is archived (coaS3Key set).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		lots, err := d.Oils.ListLots(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"lots": lots}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_lot",
		Description: "Record a supplier lot from its COA. Check list_lots first; a lot with the same supplier " +
			"and lot number is rejected. All COA values go in profile as mg/g. The app downloads the COA " +
			"link and keeps a copy; if that fails the lot is still saved and archiveError says why.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args oils.CreateLotRequest) (*mcp.CallToolResult, any, error) {
		return nil0(d.Oils.CreateLot(ctx, userID, args))
	})

	type updateLotArgs struct {
		LotID   string                `json:"lotId"   jsonschema:"the lot's lotId from list_lots"`
		Changes oils.UpdateLotRequest `json:"changes" jsonschema:"only the fields to change"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "update_lot",
		Description: "Fix or fill in a lot. Send only the fields that change; profile replaces the whole " +
			"profile. Returns affected batches whose numbers changed as a result.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateLotArgs) (*mcp.CallToolResult, any, error) {
		return nil0(d.Oils.UpdateLot(ctx, userID, args.LotID, args.Changes))
	})

	// ── Batches ──────────────────────────────────────────────────────────────

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_batches",
		Description: "List every oil batch, newest first, with total CBD and total THC (mg/g and mg/mL) and " +
			"each total's shares: the fraction from lab-backed numbers (lab) vs flower estimates (low/medium/high). " +
			"Use get_batch for the full profile.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		views, err := d.Oils.ListBatches(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		out := make([]batchSummary, 0, len(views))
		for _, v := range views {
			out = append(out, summarize(v))
		}
		return nil, map[string]any{"batches": out}, nil
	})

	type getBatchArgs struct {
		Batch string `json:"batch" jsonschema:"batch code (e.g. FS-CBD-BB-260717) or batchId"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_batch",
		Description: "Get one batch with its inputs and full calculated profile (the Record of Batch Profile): " +
			"every cannabinoid and terpene in mg/g and mg/mL, each with its lab vs estimated shares, plus totals.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getBatchArgs) (*mcp.CallToolResult, any, error) {
		return nil0(d.Oils.GetBatch(ctx, userID, args.Batch))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_batch",
		Description: "Record an oil batch Chris made. Send the code parts (the app builds the code), every input " +
			"with grams, the carrier, and the total grams of oil made. Returns the batch with its calculated " +
			"profile. If the code is taken, retry with batchDiscriminator 2 (or the next free number).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args oils.CreateBatchRequest) (*mcp.CallToolResult, any, error) {
		return nil0(d.Oils.CreateBatch(ctx, userID, args))
	})

	type updateBatchArgs struct {
		Batch   string                  `json:"batch"   jsonschema:"batch code or batchId"`
		Changes oils.UpdateBatchRequest `json:"changes" jsonschema:"only the fields to change"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "update_batch",
		Description: "Fix or fill in a batch. Send only the fields that change; inputs replaces the whole " +
			"input list. Changing code parts changes the code; blends that use this batch keep pointing at it. " +
			"Returns the new profile and the blends whose numbers changed (affected).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateBatchArgs) (*mcp.CallToolResult, any, error) {
		return nil0(d.Oils.UpdateBatch(ctx, userID, args.Batch, args.Changes))
	})

	type doseArgs struct {
		Batch    string  `json:"batch"    jsonschema:"batch code or batchId"`
		Compound string  `json:"compound" jsonschema:"total_cbd, total_thc, or a profile key such as cbd, d9thc, cbg, cbn"`
		TargetMg float64 `json:"targetMg" jsonschema:"mg of the compound wanted"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "dose",
		Description: "How much oil from a batch gives a target mg of a compound, in mL and grams. Saves nothing. " +
			"The answer is only as good as the batch's numbers, so mention the estimated share for flower oils.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args doseArgs) (*mcp.CallToolResult, any, error) {
		return nil0(d.Oils.Dose(ctx, userID, args.Batch, args.Compound, args.TargetMg))
	})
}

// nil0 adapts a (result, error) service call to a tool handler's return.
func nil0[T any](v *T, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return nil, nil, err
	}
	if v == nil {
		return nil, nil, fmt.Errorf("no result")
	}
	return nil, v, nil
}

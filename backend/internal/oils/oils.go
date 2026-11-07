// Package oils holds the rules for oil lots and batches: validation,
// uniqueness, ID/code assignment, reference checks, COA archiving and the
// calculated profile (ROBP) returned with every batch. Both the HTTP API and
// the MCP tools call this package, so the rules exist in exactly one place.
package oils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/oilcalc"
	"github.com/oklog/ulid/v2"
)

// Error kinds, for mapping to HTTP status / MCP error text.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	ErrInvalid  = errors.New("invalid")
)

type LotRepo interface {
	List(ctx context.Context, userID string) ([]model.Lot, error)
	Get(ctx context.Context, lotID string) (*model.Lot, error)
	Put(ctx context.Context, lot model.Lot) error
	Delete(ctx context.Context, lotID string) error
}

type BatchRepo interface {
	List(ctx context.Context, userID string) ([]model.Batch, error)
	Get(ctx context.Context, batchID string) (*model.Batch, error)
	Put(ctx context.Context, batch model.Batch) error
	Delete(ctx context.Context, batchID string) error
}

type PlantLookup interface {
	Get(ctx context.Context, plantID string) (*model.Plant, error)
}

// Archiver copies the document at url into storage under keyPrefix and
// returns the stored key; Remove deletes an archived copy.
type Archiver interface {
	Archive(ctx context.Context, url, keyPrefix string) (string, error)
	Remove(ctx context.Context, key string) error
}

type Service struct {
	lots     LotRepo
	batches  BatchRepo
	plants   PlantLookup
	archiver Archiver
	now      func() time.Time
}

func NewService(lots LotRepo, batches BatchRepo, plants PlantLookup, archiver Archiver) *Service {
	return &Service{lots: lots, batches: batches, plants: plants, archiver: archiver,
		now: func() time.Time { return time.Now().UTC() }}
}

// ── Requests / results ───────────────────────────────────────────────────────

type CreateLotRequest struct {
	Supplier          string        `json:"supplier"                 jsonschema:"who sold it, e.g. Red Mesa"`
	Product           string        `json:"product"                  jsonschema:"what it is, e.g. CBD isolate or broad spectrum distillate"`
	SupplierLotNumber string        `json:"supplierLotNumber"        jsonschema:"the supplier's lot or batch number printed on the jar and COA"`
	GramsPurchased    float64       `json:"gramsPurchased,omitempty" jsonschema:"grams in the jar as bought"`
	COAURL            string        `json:"coaUrl,omitempty"         jsonschema:"public link to the COA; the app downloads and keeps a copy"`
	Profile           model.Profile `json:"profile"                  jsonschema:"every number on the COA, converted to mg/g"`
	Notes             string        `json:"notes,omitempty"`
}

// UpdateLotRequest is a patch: nil fields are left unchanged. Profile, when
// given, replaces the whole profile.
type UpdateLotRequest struct {
	Supplier          *string        `json:"supplier,omitempty"`
	Product           *string        `json:"product,omitempty"`
	SupplierLotNumber *string        `json:"supplierLotNumber,omitempty"`
	GramsPurchased    *float64       `json:"gramsPurchased,omitempty"`
	COAURL            *string        `json:"coaUrl,omitempty"`
	Profile           *model.Profile `json:"profile,omitempty" jsonschema:"replaces the whole profile, so send every value, not just the changed one"`
	Notes             *string        `json:"notes,omitempty"`
}

type LotResult struct {
	Lot model.Lot `json:"lot"`
	// ArchiveError is set when the COA couldn't be copied to storage. The lot
	// is saved regardless; the link is kept and archiving is retried on the
	// next update.
	ArchiveError string `json:"archiveError,omitempty"`
	// Affected lists batches whose numbers changed because of this update.
	Affected []BatchRef `json:"affected,omitempty"`
}

type CreateBatchRequest struct {
	Type               model.BatchType    `json:"type"                         jsonschema:"DIS | ISO | FS | THC | BL"`
	SubType            string             `json:"subType,omitempty"            jsonschema:"optional code part, letters/digits, e.g. CBD or TST"`
	Discriminator      string             `json:"discriminator,omitempty"      jsonschema:"optional code part, letters/digits, e.g. BB for Berry Bliss"`
	Date               string             `json:"date,omitempty"               jsonschema:"YYYY-MM-DD the batch was made; omit if unknown (code gets 000000)"`
	BatchDiscriminator string             `json:"batchDiscriminator,omitempty" jsonschema:"only when the code is already taken, e.g. 2"`
	Inputs             []model.BatchInput `json:"inputs"                       jsonschema:"everything that went in except the carrier oil"`
	Carrier            *model.Carrier     `json:"carrier,omitempty"`
	DensityGPerMl      float64            `json:"densityGPerMl,omitempty"      jsonschema:"g/mL of the finished oil, used for mg/mL; omit for 0.95 (MCT); LouAna liquid coconut oil is 0.94"`
	TotalGrams         float64            `json:"totalGrams"                   jsonschema:"grams of finished oil made"`
	LabCOA             *model.LabCOA      `json:"labCoa,omitempty"             jsonschema:"only if this finished batch was itself lab tested"`
	Notes              string             `json:"notes,omitempty"`
}

// UpdateBatchRequest is a patch: nil fields are left unchanged. A pointer to
// "" clears an optional code part or the date (→ 000000). Inputs, Carrier
// and LabCOA, when given, replace the whole value.
type UpdateBatchRequest struct {
	Type               *model.BatchType    `json:"type,omitempty"`
	SubType            *string             `json:"subType,omitempty"`
	Discriminator      *string             `json:"discriminator,omitempty"`
	Date               *string             `json:"date,omitempty"`
	BatchDiscriminator *string             `json:"batchDiscriminator,omitempty"`
	Inputs             *[]model.BatchInput `json:"inputs,omitempty"     jsonschema:"replaces the whole input list, so send every input, not just the changed one"`
	Carrier            *model.Carrier      `json:"carrier,omitempty"`
	DensityGPerMl      *float64            `json:"densityGPerMl,omitempty"`
	TotalGrams         *float64            `json:"totalGrams,omitempty"`
	LabCOA             *model.LabCOA       `json:"labCoa,omitempty"     jsonschema:"adds or replaces the batch's own lab COA"`
	Notes              *string             `json:"notes,omitempty"`
}

type BatchRef struct {
	BatchID string `json:"batchId"`
	Code    string `json:"code"`
}

// BatchView is a batch with its calculated profile. ProfileError is set
// (and Profile nil) when the profile can't be calculated, e.g. an input was
// later made invalid -- one bad batch shouldn't fail a whole list.
type BatchView struct {
	Batch        model.Batch      `json:"batch"`
	Profile      *oilcalc.Profile `json:"profile,omitempty"`
	ProfileError string           `json:"profileError,omitempty"`
}

type BatchResult struct {
	BatchView
	ArchiveError string     `json:"archiveError,omitempty"`
	Affected     []BatchRef `json:"affected,omitempty"`
}

type DoseResult struct {
	BatchID  string           `json:"batchId"`
	Code     string           `json:"code"`
	Compound oilcalc.Compound `json:"compound"`
	TargetMg float64          `json:"targetMg"`
	Grams    float64          `json:"grams"`
	Ml       float64          `json:"ml"`
}

// ── Lots ─────────────────────────────────────────────────────────────────────

func (s *Service) ListLots(ctx context.Context, userID string) ([]model.Lot, error) {
	return s.lots.List(ctx, userID)
}

func (s *Service) GetLot(ctx context.Context, userID, lotID string) (*model.Lot, error) {
	lot, err := s.lots.Get(ctx, lotID)
	if err != nil {
		return nil, err
	}
	if lot == nil || lot.UserID != userID {
		return nil, fmt.Errorf("lot %s: %w", lotID, ErrNotFound)
	}
	return lot, nil
}

func (s *Service) CreateLot(ctx context.Context, userID string, req CreateLotRequest) (*LotResult, error) {
	now := s.now().Format(time.RFC3339)
	lot := model.Lot{
		LotID:             ulid.Make().String(),
		UserID:            userID,
		Supplier:          strings.TrimSpace(req.Supplier),
		Product:           strings.TrimSpace(req.Product),
		SupplierLotNumber: strings.TrimSpace(req.SupplierLotNumber),
		GramsPurchased:    req.GramsPurchased,
		COAURL:            strings.TrimSpace(req.COAURL),
		Profile:           req.Profile,
		Notes:             req.Notes,
		CreatedAt:         now,
	}
	if err := lot.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	existing, err := s.lots.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := checkLotUnique(lot, existing); err != nil {
		return nil, err
	}

	res := &LotResult{}
	if lot.COAURL != "" {
		lot.COAS3Key, res.ArchiveError = s.archive(ctx, lot.COAURL, "oils/lots/"+lot.LotID+"/coa")
	}
	if err := s.lots.Put(ctx, lot); err != nil {
		return nil, err
	}
	res.Lot = lot
	return res, nil
}

func (s *Service) UpdateLot(ctx context.Context, userID, lotID string, req UpdateLotRequest) (*LotResult, error) {
	lot, err := s.GetLot(ctx, userID, lotID)
	if err != nil {
		return nil, err
	}
	setString(&lot.Supplier, req.Supplier)
	setString(&lot.Product, req.Product)
	setString(&lot.SupplierLotNumber, req.SupplierLotNumber)
	if req.GramsPurchased != nil {
		lot.GramsPurchased = *req.GramsPurchased
	}
	urlChanged := false
	if req.COAURL != nil && strings.TrimSpace(*req.COAURL) != lot.COAURL {
		lot.COAURL = strings.TrimSpace(*req.COAURL)
		lot.COAS3Key = ""
		urlChanged = true
	}
	if req.Profile != nil {
		lot.Profile = *req.Profile
	}
	if req.Notes != nil {
		lot.Notes = *req.Notes
	}
	lot.UpdatedAt = s.now().Format(time.RFC3339)

	if err := lot.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	all, err := s.lots.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := checkLotUnique(*lot, all); err != nil {
		return nil, err
	}

	res := &LotResult{}
	// Archive a new link, or retry one that failed before.
	if lot.COAURL != "" && (urlChanged || lot.COAS3Key == "") {
		lot.COAS3Key, res.ArchiveError = s.archive(ctx, lot.COAURL, "oils/lots/"+lot.LotID+"/coa")
	}
	if err := s.lots.Put(ctx, *lot); err != nil {
		return nil, err
	}
	res.Lot = *lot

	if req.Profile != nil {
		batches, err := s.batches.List(ctx, userID)
		if err != nil {
			// The lot is saved; only the ripple report is missing.
			log.Printf("warn: list batches for lot %s ripple: %v", lotID, err)
		} else {
			res.Affected = refs(batches, oilcalc.New(nil, batches).LotDependents(lotID))
		}
	}
	return res, nil
}

// DeleteLot removes a lot. Refused while any batch uses it directly, since
// those batches' profiles would no longer calculate; the error names them.
func (s *Service) DeleteLot(ctx context.Context, userID, lotID string) error {
	lot, err := s.GetLot(ctx, userID, lotID)
	if err != nil {
		return err
	}
	batches, err := s.batches.List(ctx, userID)
	if err != nil {
		return err
	}
	var users []string
	for _, b := range batches {
		for _, in := range b.Inputs {
			if in.Kind == model.InputLot && in.LotID == lotID {
				users = append(users, b.Code)
				break
			}
		}
	}
	if len(users) > 0 {
		slices.Sort(users)
		return fmt.Errorf("%w: lot %s is used by %s; delete or change those batches first",
			ErrConflict, lot.SupplierLotNumber, strings.Join(users, ", "))
	}
	if err := s.lots.Delete(ctx, lotID); err != nil {
		return err
	}
	s.removeArchived(ctx, lot.COAS3Key)
	return nil
}

func checkLotUnique(lot model.Lot, existing []model.Lot) error {
	for _, e := range existing {
		if e.LotID == lot.LotID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(e.Supplier), lot.Supplier) &&
			strings.EqualFold(strings.TrimSpace(e.SupplierLotNumber), lot.SupplierLotNumber) {
			return fmt.Errorf("%w: %s lot %s is already recorded (lotId %s)",
				ErrConflict, lot.Supplier, lot.SupplierLotNumber, e.LotID)
		}
	}
	return nil
}

// ── Batches ──────────────────────────────────────────────────────────────────

func (s *Service) ListBatches(ctx context.Context, userID string) ([]BatchView, error) {
	lots, batches, err := s.loadAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	calc := oilcalc.New(lots, batches)
	views := make([]BatchView, 0, len(batches))
	for _, b := range batches {
		views = append(views, view(calc, b))
	}
	// Newest first; unknown dates last.
	slices.SortStableFunc(views, func(a, b BatchView) int {
		if c := strings.Compare(b.Batch.Date, a.Batch.Date); c != 0 {
			return c
		}
		return strings.Compare(a.Batch.Code, b.Batch.Code)
	})
	return views, nil
}

// GetBatch accepts a batch ID or a batch code (case-insensitive).
func (s *Service) GetBatch(ctx context.Context, userID, idOrCode string) (*BatchView, error) {
	lots, batches, err := s.loadAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	b, ok := findBatch(batches, idOrCode)
	if !ok {
		return nil, fmt.Errorf("batch %s: %w", idOrCode, ErrNotFound)
	}
	v := view(oilcalc.New(lots, batches), b)
	return &v, nil
}

func (s *Service) CreateBatch(ctx context.Context, userID string, req CreateBatchRequest) (*BatchResult, error) {
	density := req.DensityGPerMl
	if density == 0 {
		density = model.DefaultCarrierDensity
	}
	b := model.Batch{
		BatchID:            ulid.Make().String(),
		UserID:             userID,
		Type:               req.Type,
		SubType:            strings.TrimSpace(req.SubType),
		Discriminator:      strings.TrimSpace(req.Discriminator),
		Date:               strings.TrimSpace(req.Date),
		BatchDiscriminator: strings.TrimSpace(req.BatchDiscriminator),
		Inputs:             req.Inputs,
		Carrier:            req.Carrier,
		DensityGPerMl:      density,
		TotalGrams:         req.TotalGrams,
		LabCOA:             req.LabCOA,
		Notes:              req.Notes,
		CreatedAt:          s.now().Format(time.RFC3339),
	}
	if b.LabCOA != nil {
		b.LabCOA.S3Key = "" // only ever set by archiving
	}
	b.Inputs = slices.Clone(b.Inputs)
	for i := range b.Inputs {
		b.Inputs[i].COAS3Key = ""
	}
	return s.saveBatch(ctx, userID, b, nil, false)
}

func (s *Service) UpdateBatch(ctx context.Context, userID, idOrCode string, req UpdateBatchRequest) (*BatchResult, error) {
	_, batches, err := s.loadAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	existing, ok := findBatch(batches, idOrCode)
	if !ok {
		return nil, fmt.Errorf("batch %s: %w", idOrCode, ErrNotFound)
	}
	b := existing
	if req.Type != nil {
		b.Type = *req.Type
	}
	setString(&b.SubType, req.SubType)
	setString(&b.Discriminator, req.Discriminator)
	setString(&b.Date, req.Date)
	setString(&b.BatchDiscriminator, req.BatchDiscriminator)
	if req.Inputs != nil {
		// Keep archived copies for flower COA links that didn't change.
		b.Inputs = slices.Clone(*req.Inputs)
		for i := range b.Inputs {
			b.Inputs[i].COAS3Key = keptArchive(existing.Inputs, strings.TrimSpace(b.Inputs[i].COAURL))
		}
	}
	if req.Carrier != nil {
		b.Carrier = req.Carrier
	}
	if req.DensityGPerMl != nil {
		b.DensityGPerMl = *req.DensityGPerMl
	}
	if req.TotalGrams != nil {
		b.TotalGrams = *req.TotalGrams
	}
	labChanged := false
	if req.LabCOA != nil {
		next := *req.LabCOA
		next.S3Key = ""
		// Keep the archived copy when the link didn't change.
		if existing.LabCOA != nil && existing.LabCOA.URL == strings.TrimSpace(next.URL) {
			next.S3Key = existing.LabCOA.S3Key
		}
		b.LabCOA = &next
		labChanged = true
	}
	if req.Notes != nil {
		b.Notes = *req.Notes
	}
	b.UpdatedAt = s.now().Format(time.RFC3339)

	// Only composition changes alter other batches' mg/g. Density only
	// changes this batch's mg/mL.
	compositionChanged := req.Inputs != nil || req.TotalGrams != nil || labChanged
	return s.saveBatch(ctx, userID, b, &existing, compositionChanged)
}

// saveBatch validates, checks uniqueness and references, calculates the
// profile (which also catches cycles and missing inputs), archives any COA
// links not yet archived (lab COA, flower COAs), then writes. old is the
// batch before this update, nil when creating.
func (s *Service) saveBatch(ctx context.Context, userID string, b model.Batch, old *model.Batch, reportRipple bool) (*BatchResult, error) {
	// Own the inputs: archiving writes keys into them, and they may share a
	// backing array with old.
	b.Inputs = slices.Clone(b.Inputs)
	for i := range b.Inputs {
		b.Inputs[i].COAURL = strings.TrimSpace(b.Inputs[i].COAURL)
	}
	code, err := model.BuildBatchCode(b.Type, b.SubType, b.Discriminator, b.Date, b.BatchDiscriminator)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	b.Code = code
	if b.LabCOA != nil {
		b.LabCOA.URL = strings.TrimSpace(b.LabCOA.URL)
	}
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	lots, batches, err := s.loadAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, e := range batches {
		if e.BatchID != b.BatchID && strings.EqualFold(e.Code, b.Code) {
			return nil, fmt.Errorf("%w: batch code %s is already used (batchId %s); add a batchDiscriminator such as 2",
				ErrConflict, b.Code, e.BatchID)
		}
	}
	if err := s.checkPlants(ctx, userID, b); err != nil {
		return nil, err
	}

	// Calculate against the proposed state: this batch replacing its old
	// version (or added, if new).
	proposed := make([]model.Batch, 0, len(batches)+1)
	for _, e := range batches {
		if e.BatchID != b.BatchID {
			proposed = append(proposed, e)
		}
	}
	proposed = append(proposed, b)
	calc := oilcalc.New(lots, proposed)
	if _, err := calc.Batch(b.BatchID); err != nil {
		if errors.Is(err, oilcalc.ErrNotFound) || errors.Is(err, oilcalc.ErrCycle) {
			return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
		}
		return nil, err
	}

	res := &BatchResult{}
	var archiveErrs []string
	if b.LabCOA != nil && b.LabCOA.URL != "" && b.LabCOA.S3Key == "" {
		var msg string
		b.LabCOA.S3Key, msg = s.archive(ctx, b.LabCOA.URL, "oils/batches/"+b.BatchID+"/coa")
		if msg != "" {
			archiveErrs = append(archiveErrs, "lab COA: "+msg)
		}
	}
	for i, in := range b.Inputs {
		if in.COAURL == "" || in.COAS3Key != "" {
			continue
		}
		var msg string
		b.Inputs[i].COAS3Key, msg = s.archive(ctx, in.COAURL, "oils/batches/"+b.BatchID+"/flower-coa")
		if msg != "" {
			archiveErrs = append(archiveErrs, fmt.Sprintf("input %d (%s) COA: %s", i, in.Strain, msg))
		}
	}
	res.ArchiveError = strings.Join(archiveErrs, "; ")
	proposed[len(proposed)-1] = b
	calc = oilcalc.New(lots, proposed)

	if err := s.batches.Put(ctx, b); err != nil {
		return nil, err
	}
	// Archived copies the batch no longer references (link changed, input
	// removed) are cleaned up only after the new version is saved.
	if old != nil {
		keep := map[string]bool{}
		for _, k := range archivedKeys(b) {
			keep[k] = true
		}
		for _, k := range archivedKeys(*old) {
			if !keep[k] {
				s.removeArchived(ctx, k)
			}
		}
	}
	res.BatchView = view(calc, b)
	if old != nil && reportRipple {
		res.Affected = refs(proposed, calc.Dependents(b.BatchID))
	}
	return res, nil
}

// DeleteBatch removes a batch. Refused while another batch uses it as an
// input, since that blend's profile would no longer calculate.
func (s *Service) DeleteBatch(ctx context.Context, userID, idOrCode string) (*BatchRef, error) {
	_, batches, err := s.loadAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	b, ok := findBatch(batches, idOrCode)
	if !ok {
		return nil, fmt.Errorf("batch %s: %w", idOrCode, ErrNotFound)
	}
	var users []string
	for _, other := range batches {
		for _, in := range other.Inputs {
			if in.Kind == model.InputBatch && in.BatchID == b.BatchID {
				users = append(users, other.Code)
				break
			}
		}
	}
	if len(users) > 0 {
		slices.Sort(users)
		return nil, fmt.Errorf("%w: batch %s is an input to %s; delete or change those batches first",
			ErrConflict, b.Code, strings.Join(users, ", "))
	}
	if err := s.batches.Delete(ctx, b.BatchID); err != nil {
		return nil, err
	}
	for _, k := range archivedKeys(b) {
		s.removeArchived(ctx, k)
	}
	return &BatchRef{BatchID: b.BatchID, Code: b.Code}, nil
}

// archivedKeys lists every archived COA copy a batch references.
func archivedKeys(b model.Batch) []string {
	var keys []string
	if b.LabCOA != nil && b.LabCOA.S3Key != "" {
		keys = append(keys, b.LabCOA.S3Key)
	}
	for _, in := range b.Inputs {
		if in.COAS3Key != "" {
			keys = append(keys, in.COAS3Key)
		}
	}
	return keys
}

// keptArchive returns the archived copy of url from an earlier version of
// the inputs, so re-sending an unchanged link doesn't download it again.
func keptArchive(prev []model.BatchInput, url string) string {
	if url == "" {
		return ""
	}
	for _, in := range prev {
		if in.COAURL == url && in.COAS3Key != "" {
			return in.COAS3Key
		}
	}
	return ""
}

// removeArchived best-effort deletes an archived COA copy. The record is
// already gone; an orphaned S3 object is preferable to failing the delete.
func (s *Service) removeArchived(ctx context.Context, key string) {
	if key == "" || s.archiver == nil {
		return
	}
	if err := s.archiver.Remove(ctx, key); err != nil {
		log.Printf("warn: remove archived COA %s: %v", key, err)
	}
}

func (s *Service) checkPlants(ctx context.Context, userID string, b model.Batch) error {
	for i, in := range b.Inputs {
		if in.Kind != model.InputFlower || in.PlantID == "" {
			continue
		}
		p, err := s.plants.Get(ctx, in.PlantID)
		if err != nil {
			return err
		}
		if p == nil || p.UserID != userID {
			return fmt.Errorf("%w: input %d: plant %s not found", ErrInvalid, i, in.PlantID)
		}
	}
	return nil
}

// Dose answers "how much oil from this batch for targetMg of compound".
// compound is a profile key (cbd, d9thc, cbg, limonene, …) or total_cbd /
// total_thc. Nothing is saved.
func (s *Service) Dose(ctx context.Context, userID, idOrCode, compound string, targetMg float64) (*DoseResult, error) {
	v, err := s.GetBatch(ctx, userID, idOrCode)
	if err != nil {
		return nil, err
	}
	if v.Profile == nil {
		return nil, fmt.Errorf("%w: batch %s has no profile: %s", ErrInvalid, v.Batch.Code, v.ProfileError)
	}
	key := strings.ToLower(strings.TrimSpace(compound))
	var c *oilcalc.Compound
	switch key {
	case "total_cbd":
		c = &v.Profile.TotalCBD
	case "total_thc":
		c = &v.Profile.TotalTHC
	default:
		for i := range v.Profile.Compounds {
			if v.Profile.Compounds[i].Key == key {
				c = &v.Profile.Compounds[i]
				break
			}
		}
	}
	if c == nil {
		return nil, fmt.Errorf("%w: batch %s has no %s", ErrInvalid, v.Batch.Code, compound)
	}
	d, err := oilcalc.Dose(c.MgPerG, v.Profile.DensityGPerMl, targetMg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &DoseResult{BatchID: v.Batch.BatchID, Code: v.Batch.Code, Compound: *c,
		TargetMg: targetMg, Grams: d.Grams, Ml: d.Ml}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (s *Service) loadAll(ctx context.Context, userID string) ([]model.Lot, []model.Batch, error) {
	lots, err := s.lots.List(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	batches, err := s.batches.List(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return lots, batches, nil
}

// archive never fails the save: on error it returns "" and the reason.
func (s *Service) archive(ctx context.Context, url, prefix string) (key, errMsg string) {
	if s.archiver == nil {
		return "", "COA archiving is not configured"
	}
	key, err := s.archiver.Archive(ctx, url, prefix)
	if err != nil {
		log.Printf("warn: archive COA %s: %v", url, err)
		return "", err.Error()
	}
	return key, ""
}

func view(calc *oilcalc.Calculator, b model.Batch) BatchView {
	v := BatchView{Batch: b}
	p, err := calc.Batch(b.BatchID)
	if err != nil {
		v.ProfileError = err.Error()
		return v
	}
	v.Profile = &p
	return v
}

func findBatch(batches []model.Batch, idOrCode string) (model.Batch, bool) {
	needle := strings.TrimSpace(idOrCode)
	for _, b := range batches {
		if b.BatchID == needle || strings.EqualFold(b.Code, needle) {
			return b, true
		}
	}
	return model.Batch{}, false
}

func refs(batches []model.Batch, ids []string) []BatchRef {
	codes := make(map[string]string, len(batches))
	for _, b := range batches {
		codes[b.BatchID] = b.Code
	}
	out := make([]BatchRef, 0, len(ids))
	for _, id := range ids {
		out = append(out, BatchRef{BatchID: id, Code: codes[id]})
	}
	return out
}

func setString(dst *string, src *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}

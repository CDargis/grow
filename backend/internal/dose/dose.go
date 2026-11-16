// Package dose holds the rules for Chris's dose plan: versions (current,
// draft, past), the shelf of named sources, and the derived dose card. Used
// by the HTTP API (read) and the MCP tools (read + write). Error kinds are
// the oils package's, so the HTTP and MCP error mapping is shared.
package dose

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Lambda has no zoneinfo; "today" is Chicago's today

	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/oilcalc"
	"github.com/cdargis/grow/internal/oils"
	"github.com/oklog/ulid/v2"
)

type VersionRepo interface {
	List(ctx context.Context, userID string) ([]model.DoseVersion, error)
	Put(ctx context.Context, v model.DoseVersion) error
	Delete(ctx context.Context, versionID string) error
}

type ShelfRepo interface {
	List(ctx context.Context, userID string) ([]model.ShelfItem, error)
	Put(ctx context.Context, item model.ShelfItem) error
	Delete(ctx context.Context, shelfItemID string) error
}

type Service struct {
	versions VersionRepo
	shelf    ShelfRepo
	lots     oils.LotRepo
	batches  oils.BatchRepo
	archiver oils.Archiver
	now      func() time.Time
	loc      *time.Location
}

func NewService(versions VersionRepo, shelf ShelfRepo, lots oils.LotRepo, batches oils.BatchRepo, archiver oils.Archiver) *Service {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		// Can't happen with time/tzdata embedded; fall back rather than panic.
		log.Printf("warn: load America/Chicago: %v; using UTC", err)
		loc = time.UTC
	}
	return &Service{versions: versions, shelf: shelf, lots: lots, batches: batches, archiver: archiver,
		now: time.Now, loc: loc}
}

func (s *Service) today() string { return s.now().In(s.loc).Format("2006-01-02") }

func (s *Service) stamp() string { return s.now().UTC().Format(time.RFC3339) }

// ── Requests / views ─────────────────────────────────────────────────────────

type LineRequest struct {
	Shelf    string             `json:"shelf"          jsonschema:"shelf item name (e.g. Distillate oil) or shelfItemId; must already exist, see list_shelf"`
	Mg       float64            `json:"mg"             jsonschema:"mg of the compound for this line"`
	Compound model.DoseCompound `json:"compound"       jsonschema:"cbd or thc (both are totals: CBD + 0.877×CBDA, Δ9-THC + 0.877×THCA)"`
	Note     string             `json:"note,omitempty"`
}

type TimeRequest struct {
	Name  string        `json:"name"  jsonschema:"time of day, e.g. Morning, Midday, Night"`
	Lines []LineRequest `json:"lines"`
}

type CreateVersionRequest struct {
	Name      string        `json:"name,omitempty"      jsonschema:"optional label, e.g. Distillate-led"`
	Times     []TimeRequest `json:"times"               jsonschema:"every time of day with every line, in order"`
	Notes     string        `json:"notes,omitempty"`
	StartDate string        `json:"startDate,omitempty" jsonschema:"YYYY-MM-DD. Omit to save as an undated draft. With a current plan, a date ends it that day (today or later = switch, earlier = backfill not allowed). With no plan yet, omit for today or give the date it actually started"`
}

// UpdateVersionRequest is a patch. Times, when given, replaces every time.
type UpdateVersionRequest struct {
	VersionID string         `json:"versionId,omitempty" jsonschema:"omit for the current version (bump) or the draft if there is one, else current (amend)"`
	Mode      string         `json:"mode"                jsonschema:"bump = dose change: end the current version and start a new one with these changes. amend = edit in place, no history. If Chris didn't say which, ask him."`
	Name      *string        `json:"name,omitempty"`
	Times     *[]TimeRequest `json:"times,omitempty"     jsonschema:"replaces every time of day, so send the whole plan, not just the changed line"`
	Notes     *string        `json:"notes,omitempty"`
	StartDate string         `json:"startDate,omitempty" jsonschema:"bump only: YYYY-MM-DD the new version starts; omit for today"`
}

type StartVersionRequest struct {
	VersionID string `json:"versionId,omitempty" jsonschema:"omit for the draft"`
	Date      string `json:"date,omitempty"      jsonschema:"YYYY-MM-DD it starts; omit for today; a future date schedules it"`
}

type SetShelfItemRequest struct {
	Name    string            `json:"name"              jsonschema:"the role, e.g. Distillate oil or Night; matched case-insensitively; a new name creates the item"`
	NewName string            `json:"newName,omitempty" jsonschema:"rename the item; plan lines follow it"`
	Source  model.ShelfSource `json:"source"            jsonschema:"what fills it now"`
	Since   string            `json:"since,omitempty"   jsonschema:"YYYY-MM-DD this source started; omit for today"`
	Notes   *string           `json:"notes,omitempty"`
}

type ShelfItemView struct {
	Item     model.ShelfItem `json:"item"`
	Strength *Strength       `json:"strength,omitempty"`
	// StrengthError is set when the strength can't be worked out, e.g. the
	// batch was deleted or its profile can't be calculated.
	StrengthError string `json:"strengthError,omitempty"`
	ArchiveError  string `json:"archiveError,omitempty"`
}

// PastVersion is a past version as stored, with shelf names resolved. No
// mL: a past version's amounts depended on bottles that have since changed.
type PastVersion struct {
	Version    model.DoseVersion `json:"version"`
	ShelfNames map[string]string `json:"shelfNames"`
}

type View struct {
	Today   string          `json:"today"`
	Current *VersionView    `json:"current,omitempty"`
	Draft   *VersionView    `json:"draft,omitempty"`
	History []PastVersion   `json:"history"` // newest first
	Shelf   []ShelfItemView `json:"shelf"`
}

// ── Read ─────────────────────────────────────────────────────────────────────

type snapshot struct {
	today     string
	versions  []model.DoseVersion
	items     map[string]model.ShelfItem
	itemList  []model.ShelfItem
	strengths map[string]Strength
	errs      map[string]string
	lotLabels map[string]string
}

func (s *Service) load(ctx context.Context, userID string) (*snapshot, error) {
	versions, err := s.versions.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.shelf.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	lots, err := s.lots.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	batches, err := s.batches.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	snap := &snapshot{
		today: s.today(), versions: versions, itemList: items,
		items: map[string]model.ShelfItem{}, strengths: map[string]Strength{},
		errs: map[string]string{}, lotLabels: map[string]string{},
	}
	for _, l := range lots {
		snap.lotLabels[l.LotID] = l.Supplier + " · " + l.Product + " · " + l.SupplierLotNumber
	}
	calc := oilcalc.New(lots, batches)
	for _, it := range items {
		snap.items[it.ShelfItemID] = it
		st, err := strength(calc, it.Source)
		if err != nil {
			snap.errs[it.ShelfItemID] = err.Error()
			continue
		}
		snap.strengths[it.ShelfItemID] = st
	}
	slices.SortFunc(snap.itemList, func(a, b model.ShelfItem) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return snap, nil
}

func strength(calc *oilcalc.Calculator, src model.ShelfSource) (Strength, error) {
	if src.Kind == model.ShelfFromProduct {
		return ProductStrength(src), nil
	}
	p, err := calc.Batch(src.BatchID)
	if err != nil {
		return Strength{}, err
	}
	lg, err := calc.LotGrams(src.BatchID)
	if err != nil {
		return Strength{}, err
	}
	return BatchStrength(p, lg), nil
}

func (snap *snapshot) derive(v model.DoseVersion) *VersionView {
	vv := Derive(v, snap.today, snap.items, snap.strengths, snap.errs, snap.lotLabels)
	return &vv
}

func (snap *snapshot) shelfViews() []ShelfItemView {
	out := make([]ShelfItemView, 0, len(snap.itemList))
	for _, it := range snap.itemList {
		v := ShelfItemView{Item: it, StrengthError: snap.errs[it.ShelfItemID]}
		if st, ok := snap.strengths[it.ShelfItemID]; ok {
			v.Strength = &st
		}
		out = append(out, v)
	}
	return out
}

func (snap *snapshot) find(status model.DoseStatus) *model.DoseVersion {
	for i := range snap.versions {
		if snap.versions[i].Status(snap.today) == status {
			return &snap.versions[i]
		}
	}
	return nil
}

func (s *Service) Get(ctx context.Context, userID string) (*View, error) {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &View{Today: snap.today, History: []PastVersion{}, Shelf: snap.shelfViews()}
	if cur := snap.find(model.DoseCurrent); cur != nil {
		out.Current = snap.derive(*cur)
	}
	if d := snap.find(model.DoseDraft); d != nil {
		out.Draft = snap.derive(*d)
	}
	for _, v := range snap.versions {
		if v.Status(snap.today) != model.DosePast {
			continue
		}
		names := map[string]string{}
		for _, t := range v.Times {
			for _, l := range t.Lines {
				names[l.ShelfItemID] = snap.items[l.ShelfItemID].Name
			}
		}
		out.History = append(out.History, PastVersion{Version: v, ShelfNames: names})
	}
	slices.SortFunc(out.History, func(a, b PastVersion) int {
		return strings.Compare(b.Version.StartDate, a.Version.StartDate)
	})
	return out, nil
}

func (s *Service) ListShelf(ctx context.Context, userID string) ([]ShelfItemView, error) {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	return snap.shelfViews(), nil
}

// ── Versions ─────────────────────────────────────────────────────────────────

func (s *Service) CreateVersion(ctx context.Context, userID string, req CreateVersionRequest) (*VersionView, error) {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	if snap.find(model.DoseDraft) != nil {
		return nil, fmt.Errorf("%w: there is already a draft; amend it, start it, or delete it first", oils.ErrConflict)
	}
	times, err := snap.resolveTimes(req.Times)
	if err != nil {
		return nil, err
	}
	v := model.DoseVersion{
		VersionID: ulid.Make().String(), UserID: userID,
		Name: strings.TrimSpace(req.Name), Times: times, Notes: req.Notes,
		CreatedAt: s.stamp(),
	}
	cur := snap.find(model.DoseCurrent)
	if cur == nil && req.StartDate == "" {
		// The first plan is live from today.
		v.StartDate = snap.today
	} else {
		v.StartDate = req.StartDate
	}
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", oils.ErrInvalid, err)
	}
	if cur != nil && v.StartDate != "" {
		if err := s.handOver(ctx, cur, v.StartDate, snap.today); err != nil {
			return nil, err
		}
	}
	if err := s.versions.Put(ctx, v); err != nil {
		return nil, err
	}
	snap.versions = append(snap.versions, v)
	return snap.derive(v), nil
}

// handOver ends the current version on date, the day the next one starts.
func (s *Service) handOver(ctx context.Context, cur *model.DoseVersion, date, today string) error {
	if err := model.ValidDate(date); err != nil {
		return fmt.Errorf("%w: %w", oils.ErrInvalid, err)
	}
	if date < today {
		return fmt.Errorf("%w: start date %s is in the past; a new version can start today at the earliest", oils.ErrInvalid, date)
	}
	if date <= cur.StartDate {
		return fmt.Errorf("%w: the current version started %s; to change it on its first day, amend it instead", oils.ErrInvalid, cur.StartDate)
	}
	cur.EndDate = date
	cur.UpdatedAt = s.stamp()
	return s.versions.Put(ctx, *cur)
}

func (s *Service) UpdateVersion(ctx context.Context, userID string, req UpdateVersionRequest) (*VersionView, error) {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	switch req.Mode {
	case "amend":
		return s.amend(ctx, snap, req)
	case "bump":
		return s.bump(ctx, userID, snap, req)
	default:
		return nil, fmt.Errorf("%w: mode %q must be bump (dose change, keeps history) or amend (edit in place); ask Chris which", oils.ErrInvalid, req.Mode)
	}
}

func (s *Service) amend(ctx context.Context, snap *snapshot, req UpdateVersionRequest) (*VersionView, error) {
	if req.StartDate != "" {
		return nil, fmt.Errorf("%w: amend doesn't change dates; use start_dose_version for a draft's date", oils.ErrInvalid)
	}
	var v *model.DoseVersion
	if req.VersionID != "" {
		v = snap.byID(req.VersionID)
	} else if v = snap.find(model.DoseDraft); v == nil {
		v = snap.find(model.DoseCurrent)
	}
	if v == nil {
		return nil, fmt.Errorf("version %s: %w", req.VersionID, oils.ErrNotFound)
	}
	if err := snap.apply(v, req); err != nil {
		return nil, err
	}
	v.UpdatedAt = s.stamp()
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", oils.ErrInvalid, err)
	}
	if err := s.versions.Put(ctx, *v); err != nil {
		return nil, err
	}
	return snap.derive(*v), nil
}

func (s *Service) bump(ctx context.Context, userID string, snap *snapshot, req UpdateVersionRequest) (*VersionView, error) {
	cur := snap.find(model.DoseCurrent)
	if cur == nil {
		return nil, fmt.Errorf("%w: there is no current plan to bump; create one", oils.ErrInvalid)
	}
	if req.VersionID != "" && req.VersionID != cur.VersionID {
		return nil, fmt.Errorf("%w: only the current version can be bumped", oils.ErrInvalid)
	}
	if snap.find(model.DoseDraft) != nil {
		return nil, fmt.Errorf("%w: there is a draft waiting; start it, amend it, or delete it before bumping", oils.ErrConflict)
	}
	next := *cur
	next.VersionID = ulid.Make().String()
	next.UserID = userID
	next.StartDate = req.StartDate
	if next.StartDate == "" {
		next.StartDate = snap.today
	}
	next.EndDate = ""
	next.CreatedAt = s.stamp()
	next.UpdatedAt = ""
	if err := snap.apply(&next, req); err != nil {
		return nil, err
	}
	if err := next.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", oils.ErrInvalid, err)
	}
	if err := s.handOver(ctx, cur, next.StartDate, snap.today); err != nil {
		return nil, err
	}
	if err := s.versions.Put(ctx, next); err != nil {
		return nil, err
	}
	snap.versions = append(snap.versions, next)
	return snap.derive(next), nil
}

func (snap *snapshot) apply(v *model.DoseVersion, req UpdateVersionRequest) error {
	if req.Name != nil {
		v.Name = strings.TrimSpace(*req.Name)
	}
	if req.Notes != nil {
		v.Notes = *req.Notes
	}
	if req.Times != nil {
		times, err := snap.resolveTimes(*req.Times)
		if err != nil {
			return err
		}
		v.Times = times
	}
	return nil
}

// StartVersion makes the draft live on a date (today by default). Starting
// one that's already scheduled moves its date.
func (s *Service) StartVersion(ctx context.Context, userID string, req StartVersionRequest) (*VersionView, error) {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	var d *model.DoseVersion
	if req.VersionID != "" {
		d = snap.byID(req.VersionID)
	} else {
		d = snap.find(model.DoseDraft)
	}
	if d == nil {
		return nil, fmt.Errorf("draft %s: %w", req.VersionID, oils.ErrNotFound)
	}
	if d.Status(snap.today) != model.DoseDraft {
		return nil, fmt.Errorf("%w: that version has already started", oils.ErrInvalid)
	}
	date := req.Date
	if date == "" {
		date = snap.today
	}
	if cur := snap.find(model.DoseCurrent); cur != nil {
		if err := s.handOver(ctx, cur, date, snap.today); err != nil {
			return nil, err
		}
	} else if err := model.ValidDate(date); err != nil {
		return nil, fmt.Errorf("%w: %w", oils.ErrInvalid, err)
	}
	d.StartDate = date
	d.UpdatedAt = s.stamp()
	if err := s.versions.Put(ctx, *d); err != nil {
		return nil, err
	}
	return snap.derive(*d), nil
}

// DeleteVersion deletes a draft. Started versions are history and stay.
func (s *Service) DeleteVersion(ctx context.Context, userID, versionID string) error {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return err
	}
	var d *model.DoseVersion
	if versionID != "" {
		d = snap.byID(versionID)
	} else {
		d = snap.find(model.DoseDraft)
	}
	if d == nil {
		return fmt.Errorf("draft %s: %w", versionID, oils.ErrNotFound)
	}
	if d.Status(snap.today) != model.DoseDraft {
		return fmt.Errorf("%w: only a draft can be deleted; started versions are history", oils.ErrInvalid)
	}
	// A scheduled draft already set the current version's end date; undo it.
	if d.StartDate != "" {
		if cur := snap.find(model.DoseCurrent); cur != nil && cur.EndDate == d.StartDate {
			cur.EndDate = ""
			cur.UpdatedAt = s.stamp()
			if err := s.versions.Put(ctx, *cur); err != nil {
				return err
			}
		}
	}
	return s.versions.Delete(ctx, d.VersionID)
}

func (snap *snapshot) byID(id string) *model.DoseVersion {
	for i := range snap.versions {
		if snap.versions[i].VersionID == id {
			return &snap.versions[i]
		}
	}
	return nil
}

func (snap *snapshot) resolveTimes(req []TimeRequest) ([]model.DoseTime, error) {
	out := make([]model.DoseTime, 0, len(req))
	for _, t := range req {
		dt := model.DoseTime{Name: strings.TrimSpace(t.Name), Lines: make([]model.DoseLine, 0, len(t.Lines))}
		for _, l := range t.Lines {
			it, ok := snap.findItem(l.Shelf)
			if !ok {
				return nil, fmt.Errorf("%w: no shelf item %q; add it with set_shelf_item first", oils.ErrInvalid, l.Shelf)
			}
			dt.Lines = append(dt.Lines, model.DoseLine{
				ShelfItemID: it.ShelfItemID, Mg: l.Mg,
				Compound: model.DoseCompound(strings.ToLower(strings.TrimSpace(string(l.Compound)))),
				Note:     l.Note,
			})
		}
		out = append(out, dt)
	}
	return out, nil
}

// ── Shelf ────────────────────────────────────────────────────────────────────

func (snap *snapshot) findItem(nameOrID string) (model.ShelfItem, bool) {
	needle := strings.TrimSpace(nameOrID)
	for _, it := range snap.itemList {
		if it.ShelfItemID == needle || strings.EqualFold(it.Name, needle) {
			return it, true
		}
	}
	return model.ShelfItem{}, false
}

// SetShelfItem creates an item, or repoints/renames an existing one. A new
// source pushes the old one into history, unless it started the same day
// (then it's treated as a correction).
func (s *Service) SetShelfItem(ctx context.Context, userID string, req SetShelfItemRequest) (*ShelfItemView, error) {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	src := req.Source
	src.LegacyCOAURL, src.LegacyCOAS3Key = "", ""
	src.COAs = cleanCOAs(src.COAs)
	if src.Kind == model.ShelfFromBatch {
		// Accept a batch code as well as an ID.
		batches, err := s.batches.List(ctx, userID)
		if err != nil {
			return nil, err
		}
		found := false
		for _, b := range batches {
			if b.BatchID == strings.TrimSpace(src.BatchID) || strings.EqualFold(b.Code, strings.TrimSpace(src.BatchID)) {
				src.BatchID, found = b.BatchID, true
				break
			}
		}
		if !found && src.BatchID != "" {
			return nil, fmt.Errorf("batch %s: %w", src.BatchID, oils.ErrNotFound)
		}
	}
	since := req.Since
	if since == "" {
		since = snap.today
	}

	item, exists := snap.findItem(req.Name)
	// COA links already archived keep their copy, whatever else changed.
	archived := map[string]string{}
	for _, c := range item.Source.COAs {
		if c.S3Key != "" {
			archived[c.URL] = c.S3Key
		}
	}
	if !exists {
		item = model.ShelfItem{
			ShelfItemID: ulid.Make().String(), UserID: userID,
			Name: strings.TrimSpace(req.Name), Source: src, Since: since,
			CreatedAt: s.stamp(),
		}
	} else {
		old := item.Source
		old.COAs = cleanCOAs(old.COAs)
		if !reflect.DeepEqual(old, src) {
			switch {
			case since < item.Since:
				return nil, fmt.Errorf("%w: since %s is before the current source started (%s)", oils.ErrInvalid, since, item.Since)
			case since > item.Since:
				item.History = append(item.History, model.ShelfHistoryEntry{Source: item.Source, From: item.Since, To: since})
			}
			item.Since = since
		}
		item.Source = src
		item.UpdatedAt = s.stamp()
	}
	if req.NewName != "" {
		item.Name = strings.TrimSpace(req.NewName)
	}
	if req.Notes != nil {
		item.Notes = *req.Notes
	}
	for _, other := range snap.itemList {
		if other.ShelfItemID != item.ShelfItemID && strings.EqualFold(other.Name, item.Name) {
			return nil, fmt.Errorf("%w: shelf item %q", oils.ErrConflict, item.Name)
		}
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", oils.ErrInvalid, err)
	}

	view := ShelfItemView{}
	var archiveErrs []string
	for i := range item.Source.COAs {
		c := &item.Source.COAs[i]
		if key := archived[c.URL]; key != "" {
			c.S3Key = key
			continue
		}
		if s.archiver == nil {
			archiveErrs = append(archiveErrs, "COA archiving is not configured")
			break
		}
		key, err := s.archiver.Archive(ctx, c.URL, "dose/shelf/"+item.ShelfItemID+"/coa")
		if err != nil {
			log.Printf("warn: archive COA %s: %v", c.URL, err)
			archiveErrs = append(archiveErrs, fmt.Sprintf("%s: %v", c.URL, err))
			continue
		}
		c.S3Key = key
	}
	view.ArchiveError = strings.Join(archiveErrs, "; ")
	if err := s.shelf.Put(ctx, item); err != nil {
		return nil, err
	}

	// Report the strength as it now stands.
	snap.items[item.ShelfItemID] = item
	lots, err := s.lots.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	batches, err := s.batches.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	view.Item = item
	if st, err := strength(oilcalc.New(lots, batches), item.Source); err != nil {
		view.StrengthError = err.Error()
	} else {
		view.Strength = &st
	}
	return &view, nil
}

// cleanCOAs trims links and drops archive keys, so a source from input
// compares equal to the stored one when only the archive differs.
func cleanCOAs(in []model.ShelfCOA) []model.ShelfCOA {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.ShelfCOA, len(in))
	for i, c := range in {
		out[i] = model.ShelfCOA{URL: strings.TrimSpace(c.URL), Label: strings.TrimSpace(c.Label)}
	}
	return out
}

// DeleteShelfItem is refused while any version (past ones included, which
// need the name for history) has a line on it.
func (s *Service) DeleteShelfItem(ctx context.Context, userID, nameOrID string) error {
	snap, err := s.load(ctx, userID)
	if err != nil {
		return err
	}
	it, ok := snap.findItem(nameOrID)
	if !ok {
		return fmt.Errorf("shelf item %s: %w", nameOrID, oils.ErrNotFound)
	}
	for _, v := range snap.versions {
		for _, t := range v.Times {
			for _, l := range t.Lines {
				if l.ShelfItemID == it.ShelfItemID {
					return fmt.Errorf("%w: %q is used by a plan version (started %s); repoint it instead",
						oils.ErrConflict, it.Name, orDraft(v.StartDate))
				}
			}
		}
	}
	return s.shelf.Delete(ctx, it.ShelfItemID)
}

func orDraft(date string) string {
	if date == "" {
		return "draft"
	}
	return date
}

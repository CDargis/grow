package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/store"
)

type app struct {
	plants       *store.PlantStore
	logs         *store.LogStore
	milestones   *store.MilestoneStore
	observations *store.ObservationStore
	s3           *s3.Client
	mediaBkt     string
	userID       string
	anthropicKey string
}

// ── Claude API ────────────────────────────────────────────────────────────────

type claudeImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg"
	Data      string `json:"data"`
}

type claudeContentBlock struct {
	Type   string             `json:"type"`             // "text" or "image"
	Text   string             `json:"text,omitempty"`
	Source *claudeImageSource `json:"source,omitempty"`
}

type claudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []claudeContentBlock
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeResponseBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeResponse struct {
	Content []claudeResponseBlock `json:"content"`
}

type milestonePredict struct {
	Type          string `json:"type"`
	PredictedDate string `json:"predicted_date"`
	Confidence    string `json:"confidence"`
	Notes         string `json:"notes,omitempty"`
}

type observationPredict struct {
	Category       string `json:"category"`
	Text           string `json:"text"`
	RequiresAction bool   `json:"requires_action"`
}

type recalResponse struct {
	Milestones   []milestonePredict   `json:"milestones"`
	Observations []observationPredict `json:"observations"`
	Reason       string               `json:"reason"`
}

const systemPrompt = `You are an AI assistant helping track cannabis plant growth milestones.
Given information about a cannabis plant, predict upcoming milestone dates and note any observations.

Respond with ONLY valid JSON (no markdown fences) matching this exact schema:
{
  "milestones": [
    {
      "type": "<see valid types below>",
      "predicted_date": "YYYY-MM-DD",
      "confidence": "low|medium|high",
      "notes": "optional brief explanation"
    }
  ],
  "observations": [
    {
      "category": "health|growth|pest|nutrient|general",
      "text": "brief observation (1-2 sentences)",
      "requires_action": false
    }
  ],
  "reason": "one sentence summary of reasoning"
}

Valid milestone types and typical timing from sprout/germination:
- cotyledons_off      — cotyledons yellowing/falling off (~10-21 days)
- leaf_set_1          — 1st true leaf set visible (~7-12 days)
- leaf_set_2          — 2nd true leaf set (~14-20 days)
- leaf_set_3          — 3rd true leaf set (~20-28 days)
- leaf_set_4          — 4th true leaf set, stop predicting leaf sets after this (~28-40 days)
- early_veg           — 3-4 nodes, transition from seedling (~3-5 weeks from sprout)
- full_veg            — 5+ nodes, vigorous growth established (~5-8 weeks from sprout)
- pre_flower          — first pistils/sex signs at nodes; for autos this happens automatically (~4-7 weeks), for photos it's before the flip
- flip_to_flower      — photoperiod only: light schedule change to 12/12 (grower decision, typically after 4-8 weeks veg)
- peak_flower         — full canopy coverage, trichomes forming (~weeks 4-6 of flower)
- harvest             — trichomes amber/milky, pistils receding (~weeks 8-10 photo flower, 10-14 auto from sprout)
- dry_complete        — buds dry enough to jar (~7-14 days after harvest)
- cure_ready          — properly cured, burping complete (~2-6 weeks after jarring)

Rules:
- Only predict milestones that are STILL IN THE FUTURE based on today's date and current phase.
- Skip milestones that have clearly already passed (e.g., don't predict leaf_set_1 if the plant is already in veg).
- For autoflowers: skip flip_to_flower entirely. Adjust harvest timing (~70-100 days from sprout).
- For photoperiods: include flip_to_flower if still in veg or seedling.
- Only predict leaf sets if the plant is currently in germination or seedling phase.
- Keep observations sparse — only note something if the log data provides a meaningful signal.
- Be conservative with confidence; prefer low/medium over false high confidence.
- If a photo is provided, use it as the PRIMARY signal for current growth stage. Override text-based estimates if the image clearly shows a different stage. Don't predict milestones the photo shows have already passed.`

func (a *app) fetchImageBase64(ctx context.Context, key string) (data, mediaType string, err error) {
	out, err := a.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.mediaBkt),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", "", fmt.Errorf("get object: %w", err)
	}
	defer out.Body.Close()
	raw, err := io.ReadAll(out.Body)
	if err != nil {
		return "", "", fmt.Errorf("read body: %w", err)
	}
	mt := "image/jpeg"
	if out.ContentType != nil && *out.ContentType != "" {
		mt = *out.ContentType
	}
	return base64.StdEncoding.EncodeToString(raw), mt, nil
}

func (a *app) callClaude(ctx context.Context, prompt, photoKey string) (*recalResponse, error) {
	var content interface{}
	if photoKey != "" && a.s3 != nil {
		imgData, mediaType, err := a.fetchImageBase64(ctx, photoKey)
		if err != nil {
			log.Printf("warn: fetch photo for claude: %v", err)
			content = prompt
		} else {
			content = []claudeContentBlock{
				{Type: "image", Source: &claudeImageSource{Type: "base64", MediaType: mediaType, Data: imgData}},
				{Type: "text", Text: prompt},
			}
		}
	} else {
		content = prompt
	}

	reqBody, _ := json.Marshal(claudeRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 2000,
		System:    systemPrompt,
		Messages:  []claudeMessage{{Role: "user", Content: content}},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.anthropicKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude status %d", resp.StatusCode)
	}

	var cr claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode claude response: %w", err)
	}
	if len(cr.Content) == 0 || cr.Content[0].Text == "" {
		return nil, fmt.Errorf("empty claude response")
	}

	text := strings.TrimSpace(cr.Content[0].Text)
	// Strip accidental markdown fences
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	var result recalResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse claude JSON (%s): %w", text[:min(len(text), 200)], err)
	}
	return &result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Prompt builder ────────────────────────────────────────────────────────────

// buildPrompt returns the text prompt and the S3 key of the most recent photo log (if any).
func buildPrompt(plant *model.Plant, logs []model.Log, now time.Time) (string, string) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plant: %s\n", plant.Name))
	sb.WriteString(fmt.Sprintf("Strain: %s\n", plant.Strain))
	if plant.PlantType != "" {
		sb.WriteString(fmt.Sprintf("Type: %s\n", plant.PlantType))
	}
	if plant.Genetics != "" {
		sb.WriteString(fmt.Sprintf("Genetics: %s\n", plant.Genetics))
	}
	sb.WriteString(fmt.Sprintf("Current Phase: %s (since %s)\n", plant.Phase, plant.PhaseStartDate))
	sb.WriteString(fmt.Sprintf("Today: %s\n", now.Format("2006-01-02")))

	sb.WriteString("\nPhase History:\n")
	hasPhaseHistory := false
	for i := len(logs) - 1; i >= 0; i-- {
		l := logs[i]
		if l.LogType != model.LogPhaseChange {
			continue
		}
		var d model.PhaseChangeData
		if err := json.Unmarshal(l.Data, &d); err != nil {
			continue
		}
		if d.FromPhase != "" {
			sb.WriteString(fmt.Sprintf("  %s: %s → %s\n", l.Date, d.FromPhase, d.ToPhase))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: started as %s\n", l.Date, d.ToPhase))
		}
		hasPhaseHistory = true
	}
	if !hasPhaseHistory {
		sb.WriteString("  (no phase history recorded)\n")
	}

	cutoff := now.AddDate(0, 0, -30).Format("2006-01-02")
	sb.WriteString("\nRecent Care (last 30 days):\n")
	careCount := 0
	latestPhotoKey := ""
	latestPhotoDate := ""
	for i := len(logs) - 1; i >= 0; i-- {
		l := logs[i]
		// Track most recent photo regardless of cutoff
		if l.LogType == model.LogPhoto {
			var d model.PhotoData
			if err := json.Unmarshal(l.Data, &d); err == nil && d.PhotoKey != "" {
				if latestPhotoDate == "" || l.LoggedAt > latestPhotoDate {
					latestPhotoKey = d.PhotoKey
					latestPhotoDate = l.LoggedAt
				}
			}
		}
		if l.Date < cutoff {
			continue
		}
		switch l.LogType {
		case model.LogWatering:
			var d model.WateringData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				if d.PH != 0 {
					sb.WriteString(fmt.Sprintf("  %s: watering pH=%.1f\n", l.Date, d.PH))
				} else {
					sb.WriteString(fmt.Sprintf("  %s: watering\n", l.Date))
				}
				careCount++
			}
		case model.LogFeeding:
			var d model.FeedingData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				sb.WriteString(fmt.Sprintf("  %s: feeding (pH=%.1f)\n", l.Date, d.PH))
				careCount++
			}
		case model.LogHeight:
			var d model.HeightData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				sb.WriteString(fmt.Sprintf("  %s: height %.0f%s\n", l.Date, d.Height, d.Unit))
				careCount++
			}
		case model.LogTransplant:
			var d model.TransplantData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				sb.WriteString(fmt.Sprintf("  %s: transplant → %s\n", l.Date, d.PotSize))
				careCount++
			}
		case model.LogNote:
			var d model.NoteData
			if err := json.Unmarshal(l.Data, &d); err == nil && d.Text != "" {
				text := d.Text
				if len(text) > 80 {
					text = text[:80] + "…"
				}
				sb.WriteString(fmt.Sprintf("  %s: note: %s\n", l.Date, text))
				careCount++
			}
		}
	}
	if careCount == 0 {
		sb.WriteString("  (no recent care logs)\n")
	}
	if latestPhotoKey != "" {
		sb.WriteString("\nA recent photo is attached. Use it as the primary signal for current growth stage.\n")
	}

	return sb.String(), latestPhotoKey
}

// ── Handler ───────────────────────────────────────────────────────────────────

func (a *app) handle(ctx context.Context) error {
	plants, err := a.plants.List(ctx, a.userID)
	if err != nil {
		return fmt.Errorf("list plants: %w", err)
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	for _, plant := range plants {
		if plant.Phase == model.PhaseArchived || plant.Phase == model.PhaseDead {
			continue
		}

		logs, err := a.logs.ListForPlant(ctx, plant.PlantID)
		if err != nil {
			log.Printf("warn: get logs for %s: %v", plant.PlantID, err)
			continue
		}

		prompt, photoKey := buildPrompt(&plant, logs, now)
		result, err := a.callClaude(ctx, prompt, photoKey)
		if err != nil {
			log.Printf("warn: claude for %s: %v", plant.PlantID, err)
			continue
		}

		// Upsert milestones
		for _, mp := range result.Milestones {
			confidence := model.MilestoneConfidence(mp.Confidence)
			if confidence != model.ConfidenceLow && confidence != model.ConfidenceMedium && confidence != model.ConfidenceHigh {
				confidence = model.ConfidenceLow
			}
			m := model.Milestone{
				PlantID:       plant.PlantID,
				MilestoneType: model.MilestoneType(mp.Type),
				PredictedDate: mp.PredictedDate,
				Confidence:    confidence,
				Status:        model.MilestoneStatusPredicted,
				Notes:         mp.Notes,
			}
			if err := a.milestones.Upsert(ctx, m); err != nil {
				log.Printf("warn: upsert milestone %s/%s: %v", plant.PlantID, mp.Type, err)
				continue
			}
			h := model.MilestoneRecalibrationHistory{
				PlantID:        plant.PlantID,
				RecalSK:        fmt.Sprintf("recal#%s#%s", nowStr, mp.Type),
				MilestoneType:  model.MilestoneType(mp.Type),
				PredictedDate:  mp.PredictedDate,
				Confidence:     confidence,
				RecalibratedAt: nowStr,
				Reason:         result.Reason,
			}
			if err := a.milestones.AppendHistory(ctx, h); err != nil {
				log.Printf("warn: append history %s/%s: %v", plant.PlantID, mp.Type, err)
			}
		}

		// Write observations
		for _, op := range result.Observations {
			cat := model.ObservationCategory(op.Category)
			obs := model.PlantObservation{
				Category:       cat,
				Text:           op.Text,
				RequiresAction: op.RequiresAction,
			}
			if _, err := a.observations.Create(ctx, plant.PlantID, obs); err != nil {
				log.Printf("warn: create observation %s: %v", plant.PlantID, err)
			}
		}

		log.Printf("recalibrated %s (%s): %d milestones, %d observations",
			plant.Name, plant.PlantID, len(result.Milestones), len(result.Observations))
	}

	return nil
}

func main() {
	ctx := context.Background()
	clients, err := store.NewClients(ctx)
	if err != nil {
		log.Fatalf("init clients: %v", err)
	}

	a := &app{
		plants: store.NewPlantStore(clients.DDB, os.Getenv("PLANTS_TABLE")),
		logs:   store.NewLogStore(clients.DDB, os.Getenv("LOGS_TABLE"), os.Getenv("LOGS_DATE_GSI")),
		milestones: store.NewMilestoneStore(
			clients.DDB,
			os.Getenv("MILESTONES_TABLE"),
			os.Getenv("MILESTONES_HISTORY_TABLE"),
		),
		observations: store.NewObservationStore(clients.DDB, os.Getenv("OBSERVATIONS_TABLE")),
		s3:           clients.S3,
		mediaBkt:     os.Getenv("MEDIA_BUCKET"),
		userID:       getEnvOrDefault("USER_ID", "default"),
		anthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
	}

	lambda.Start(a.handle)
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

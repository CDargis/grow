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

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/store"
)

type app struct {
	plants       *store.PlantStore
	logs         *store.LogStore
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

type observationPredict struct {
	Category     string   `json:"category"`
	Text         string   `json:"text"`
	SourceLogIds []string `json:"source_log_ids,omitempty"`
}

type recalResponse struct {
	Observations []observationPredict `json:"observations"`
}

const systemPrompt = `You are an AI assistant helping track cannabis plant growth.
Given information about a cannabis plant and its recent care logs, note any meaningful observations.

Respond with ONLY valid JSON (no markdown fences) matching this exact schema:
{
  "observations": [
    {
      "category": "health|growth|pest|nutrient|general",
      "text": "brief observation (1-2 sentences)",
      "source_log_ids": ["logId1", "logId2"]
    }
  ]
}

Rules:
- Keep observations sparse — only note something if the log data provides a meaningful signal.
- Always include source_log_ids listing the specific log IDs your observation is based on.
- If a photo is provided, use it as the primary signal for current growth stage.
- Respond with JSON only, starting with {`

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
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		result, err := a.doCallClaude(ctx, prompt, photoKey)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Printf("warn: claude attempt %d failed: %v", attempt+1, err)
	}
	return nil, lastErr
}

func (a *app) doCallClaude(ctx context.Context, prompt, photoKey string) (*recalResponse, error) {
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
					sb.WriteString(fmt.Sprintf("  [%s] %s: watering pH=%.1f\n", l.LogID, l.Date, d.PH))
				} else {
					sb.WriteString(fmt.Sprintf("  [%s] %s: watering\n", l.LogID, l.Date))
				}
				careCount++
			}
		case model.LogFeeding:
			var d model.FeedingData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				sb.WriteString(fmt.Sprintf("  [%s] %s: feeding (pH=%.1f)\n", l.LogID, l.Date, d.PH))
				careCount++
			}
		case model.LogHeight:
			var d model.HeightData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				sb.WriteString(fmt.Sprintf("  [%s] %s: height %.0f%s\n", l.LogID, l.Date, d.Height, d.Unit))
				careCount++
			}
		case model.LogTransplant:
			var d model.TransplantData
			if err := json.Unmarshal(l.Data, &d); err == nil {
				sb.WriteString(fmt.Sprintf("  [%s] %s: transplant → %s\n", l.LogID, l.Date, d.PotSize))
				careCount++
			}
		case model.LogNote:
			var d model.NoteData
			if err := json.Unmarshal(l.Data, &d); err == nil && d.Text != "" {
				text := d.Text
				if len(text) > 80 {
					text = text[:80] + "…"
				}
				sb.WriteString(fmt.Sprintf("  [%s] %s: note: %s\n", l.LogID, l.Date, text))
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

// ── Per-plant processing ──────────────────────────────────────────────────────

func (a *app) processPlant(ctx context.Context, plant model.Plant, now time.Time, nowStr string) {
	logs, err := a.logs.ListForPlant(ctx, plant.PlantID)
	if err != nil {
		log.Printf("warn: get logs for %s: %v", plant.PlantID, err)
		return
	}

	prompt, photoKey := buildPrompt(&plant, logs, now)
	result, err := a.callClaude(ctx, prompt, photoKey)
	if err != nil {
		log.Printf("warn: claude for %s: %v", plant.PlantID, err)
		return
	}

	if err := a.observations.DeleteAll(ctx, plant.PlantID); err != nil {
		log.Printf("warn: delete observations %s: %v", plant.PlantID, err)
	}
	for _, op := range result.Observations {
		obs := model.PlantObservation{
			Category:     model.ObservationCategory(op.Category),
			Text:         op.Text,
			SourceLogIds: op.SourceLogIds,
		}
		if _, err := a.observations.Create(ctx, plant.PlantID, obs); err != nil {
			log.Printf("warn: create observation %s: %v", plant.PlantID, err)
		}
	}

	if err := a.plants.UpdateLastCalibratedAt(ctx, plant.PlantID, nowStr); err != nil {
		log.Printf("warn: update lastCalibratedAt for %s: %v", plant.PlantID, err)
	}
	if err := a.plants.SetObservationsDismissed(ctx, plant.PlantID, false); err != nil {
		log.Printf("warn: clear observationsDismissed for %s: %v", plant.PlantID, err)
	}

	log.Printf("recalibrated %s (%s): %d observations",
		plant.Name, plant.PlantID, len(result.Observations))
}

// ── Handler ───────────────────────────────────────────────────────────────────

func (a *app) handle(ctx context.Context, event events.SQSEvent) error {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Collect target plantIDs from message bodies; empty = process all
	var targetPlantIDs []string
	for _, record := range event.Records {
		var msg struct {
			PlantID string `json:"plantId"`
		}
		if err := json.Unmarshal([]byte(record.Body), &msg); err == nil && msg.PlantID != "" {
			targetPlantIDs = append(targetPlantIDs, msg.PlantID)
		}
	}

	if len(targetPlantIDs) > 0 {
		for _, pid := range targetPlantIDs {
			plant, err := a.plants.Get(ctx, pid)
			if err != nil {
				log.Printf("warn: get plant %s: %v", pid, err)
				continue
			}
			if plant == nil {
				log.Printf("warn: plant not found: %s", pid)
				continue
			}
			if plant.Phase == model.PhaseArchived || plant.Phase == model.PhaseDead {
				continue
			}
			a.processPlant(ctx, *plant, now, nowStr)
		}
	} else {
		plants, err := a.plants.List(ctx, a.userID)
		if err != nil {
			return fmt.Errorf("list plants: %w", err)
		}
		for _, plant := range plants {
			if plant.Phase == model.PhaseArchived || plant.Phase == model.PhaseDead {
				continue
			}
			a.processPlant(ctx, plant, now, nowStr)
		}
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
		plants:       store.NewPlantStore(clients.DDB, os.Getenv("PLANTS_TABLE")),
		logs:         store.NewLogStore(clients.DDB, os.Getenv("LOGS_TABLE"), os.Getenv("LOGS_DATE_GSI")),
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

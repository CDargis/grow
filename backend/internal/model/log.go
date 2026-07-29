package model

import "encoding/json"

type LogType string

const (
	LogWatering           LogType = "watering"
	LogFeeding            LogType = "feeding"
	LogTraining           LogType = "training"
	LogTrimming           LogType = "trimming"
	LogSprout             LogType = "sprout"
	LogTransplant         LogType = "transplant"
	LogHeight             LogType = "height"
	LogNote               LogType = "note"
	LogPhoto              LogType = "photo"
	LogPhaseChange        LogType = "phase_change"
	LogEnvironmentChange  LogType = "environment_change"
	LogLightingChange     LogType = "lighting_change"
	LogVPDChange          LogType = "vpd_change"
)

type Log struct {
	PlantID     string          `dynamodbav:"plantId"     json:"plantId"`
	LogID       string          `dynamodbav:"logId"       json:"logId"`
	UserID      string          `dynamodbav:"userId"      json:"userId"`
	LogType     LogType         `dynamodbav:"logType"     json:"logType"`
	Date        string          `dynamodbav:"date"        json:"date"`
	LoggedAt    string          `dynamodbav:"loggedAt"    json:"loggedAt"`
	LogTypeDate string          `dynamodbav:"logTypeDate" json:"-"`
	Data        json.RawMessage `dynamodbav:"data"        json:"data"`
}

type LastActivityItem struct {
	PlantID  string  `json:"plantId"`
	LogType  LogType `json:"logType"`
	Date     string  `json:"date"`
	LoggedAt string  `json:"loggedAt"`
}

type CreateLogRequest struct {
	LogType  LogType         `json:"logType"`
	Date     string          `json:"date,omitempty"`
	LoggedAt string          `json:"loggedAt,omitempty"`
	Data     json.RawMessage `json:"data"`
}

// ── Log data shapes (for validation / documentation) ─────────────────────────

type WateringData struct {
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
	PH     float64 `json:"ph,omitempty"`
	Runoff float64 `json:"runoff,omitempty"`
}

type NutrientEntry struct {
	Name      string   `json:"name"`
	Amount    float64  `json:"amount"`
	Unit      string   `json:"unit"`
	ProductID string   `json:"productId,omitempty"`
	NPK       *NPK     `json:"npk,omitempty"`
	PctOfDose *float64 `json:"pctOfDose,omitempty"`
}

type FeedingData struct {
	Nutrients []NutrientEntry `json:"nutrients"`
	PH        float64         `json:"ph,omitempty"`
	TotalVol  float64         `json:"totalVol,omitempty"`
}

type TrainingData struct {
	Method   string `json:"method"`
	Notes    string `json:"notes,omitempty"`
	PhotoKey string `json:"photoKey,omitempty"`
}

type TrimmingData struct {
	Method   string `json:"method,omitempty"`
	Notes    string `json:"notes,omitempty"`
	PhotoKey string `json:"photoKey,omitempty"`
}

type TransplantData struct {
	PotSize string `json:"potSize"`
	Medium  string `json:"medium,omitempty"`
}

type HeightData struct {
	Height float64 `json:"height"`
	Unit   string  `json:"unit"`
}

type NoteData struct {
	Text string `json:"text"`
}

type PhotoData struct {
	PhotoKeys []string `json:"photoKeys"`
	Caption   string   `json:"caption,omitempty"`
}

type PhaseChangeData struct {
	FromPhase PlantPhase `json:"fromPhase,omitempty"`
	ToPhase   PlantPhase `json:"toPhase"`
}

type EnvironmentChangeData struct {
	FromEnvironmentID string `json:"fromEnvironmentId,omitempty"`
	ToEnvironmentID   string `json:"toEnvironmentId,omitempty"`
}

type LightingChangeData struct {
	FromSchedule string `json:"fromSchedule,omitempty"`
	ToSchedule   string `json:"toSchedule,omitempty"`
}

type VPDChangeData struct {
	FromVPD *float64 `json:"fromVpd,omitempty"`
	ToVPD   *float64 `json:"toVpd,omitempty"`
}

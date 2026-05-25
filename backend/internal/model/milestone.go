package model

type MilestoneType string

const (
	MilestoneCotyledonsOff MilestoneType = "cotyledons_off"
	MilestoneLeafSet1      MilestoneType = "leaf_set_1"
	MilestoneLeafSet2      MilestoneType = "leaf_set_2"
	MilestoneLeafSet3      MilestoneType = "leaf_set_3"
	MilestoneLeafSet4      MilestoneType = "leaf_set_4"
	MilestoneEarlyVeg      MilestoneType = "early_veg"
	MilestoneFullVeg       MilestoneType = "full_veg"
	MilestonePreFlower     MilestoneType = "pre_flower"
	MilestoneFlipToFlower  MilestoneType = "flip_to_flower"
	MilestonePeakFlower    MilestoneType = "peak_flower"
	MilestoneHarvest       MilestoneType = "harvest"
	MilestoneDryComplete   MilestoneType = "dry_complete"
	MilestoneCureReady     MilestoneType = "cure_ready"
)

type MilestoneStatus string

const (
	MilestoneStatusPredicted MilestoneStatus = "predicted"
	MilestoneStatusConfirmed MilestoneStatus = "confirmed"
	MilestoneStatusSkipped   MilestoneStatus = "skipped"
)

type MilestoneConfidence string

const (
	ConfidenceLow    MilestoneConfidence = "low"
	ConfidenceMedium MilestoneConfidence = "medium"
	ConfidenceHigh   MilestoneConfidence = "high"
)

type Milestone struct {
	PlantID          string              `dynamodbav:"plantId"          json:"plantId"`
	MilestoneType    MilestoneType       `dynamodbav:"milestoneType"    json:"milestoneType"`
	PredictedDate    string              `dynamodbav:"predictedDate"    json:"predictedDate"`
	Confidence       MilestoneConfidence `dynamodbav:"confidence"       json:"confidence"`
	ConfirmedDate    string              `dynamodbav:"confirmedDate"    json:"confirmedDate,omitempty"`
	Status           MilestoneStatus     `dynamodbav:"status"           json:"status"`
	UpdatedAt        string              `dynamodbav:"updatedAt"        json:"updatedAt"`
	Notes            string              `dynamodbav:"notes"            json:"notes,omitempty"`
	LastChangedAt    string              `dynamodbav:"lastChangedAt"    json:"lastChangedAt,omitempty"`
	LastChangedFrom  string              `dynamodbav:"lastChangedFrom"  json:"lastChangedFrom,omitempty"`
	LastChangeReason string              `dynamodbav:"lastChangeReason" json:"lastChangeReason,omitempty"`
}

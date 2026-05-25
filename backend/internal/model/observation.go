package model

type ObservationCategory string

const (
	ObservationHealth   ObservationCategory = "health"
	ObservationGrowth   ObservationCategory = "growth"
	ObservationPest     ObservationCategory = "pest"
	ObservationNutrient ObservationCategory = "nutrient"
	ObservationGeneral  ObservationCategory = "general"
)

type PlantObservation struct {
	PlantID       string              `dynamodbav:"plantId"       json:"plantId"`
	ObservationID string              `dynamodbav:"observationId" json:"observationId"`
	ObservedAt    string              `dynamodbav:"observedAt"    json:"observedAt"`
	Category      ObservationCategory `dynamodbav:"category"      json:"category"`
	Text          string              `dynamodbav:"text"          json:"text"`
	SourceLogIds  []string            `dynamodbav:"sourceLogIds"  json:"sourceLogIds,omitempty"`
}

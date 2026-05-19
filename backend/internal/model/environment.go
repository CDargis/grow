package model

type EnvironmentType string

const (
	EnvTent       EnvironmentType = "tent"
	EnvOutdoor    EnvironmentType = "outdoor"
	EnvGarage     EnvironmentType = "garage"
	EnvBasement   EnvironmentType = "basement"
	EnvRoom       EnvironmentType = "room"
	EnvGreenhouse EnvironmentType = "greenhouse"
	EnvOther      EnvironmentType = "other"
)

type Environment struct {
	EnvironmentID   string          `dynamodbav:"environmentId"   json:"environmentId"`
	UserID          string          `dynamodbav:"userId"          json:"userId"`
	Name            string          `dynamodbav:"name"            json:"name"`
	Type            EnvironmentType `dynamodbav:"type"            json:"type"`
	LightSchedule   string          `dynamodbav:"lightSchedule"   json:"lightSchedule,omitempty"`
	TargetTempF     float64         `dynamodbav:"targetTempF"     json:"targetTempF,omitempty"`
	TargetHumidity  float64         `dynamodbav:"targetHumidity"  json:"targetHumidity,omitempty"`
	PhotoKey        string          `dynamodbav:"photoKey"        json:"photoKey,omitempty"`
	CreatedAt       string          `dynamodbav:"createdAt"       json:"createdAt"`
}

type CreateEnvironmentRequest struct {
	Name           string          `json:"name"`
	Type           EnvironmentType `json:"type"`
	LightSchedule  string          `json:"lightSchedule,omitempty"`
	TargetTempF    float64         `json:"targetTempF,omitempty"`
	TargetHumidity float64         `json:"targetHumidity,omitempty"`
	PhotoKey       string          `json:"photoKey,omitempty"`
}

type UpdateEnvironmentRequest struct {
	Name           *string          `json:"name,omitempty"`
	Type           *EnvironmentType `json:"type,omitempty"`
	LightSchedule  *string          `json:"lightSchedule,omitempty"`
	TargetTempF    *float64         `json:"targetTempF,omitempty"`
	TargetHumidity *float64         `json:"targetHumidity,omitempty"`
	PhotoKey       *string          `json:"photoKey,omitempty"`
}

package model

type PlantPhase string

const (
	PhaseGermination PlantPhase = "germination"
	PhaseSeedling    PlantPhase = "seedling"
	PhaseVeg         PlantPhase = "veg"
	PhaseFlower      PlantPhase = "flower"
	PhaseHarvest     PlantPhase = "harvest"
	PhaseDrying      PlantPhase = "drying"
	PhaseCuring      PlantPhase = "curing"
	PhaseArchived    PlantPhase = "archived"
	PhaseDead        PlantPhase = "dead"
)

type PlantType string

const (
	PlantTypeAutoflower  PlantType = "autoflower"
	PlantTypePhotoperiod PlantType = "photoperiod"
	PlantTypeUnknown     PlantType = "unknown"
)

type Plant struct {
	PlantID        string     `dynamodbav:"plantId"        json:"plantId"`
	UserID         string     `dynamodbav:"userId"         json:"userId"`
	Name           string     `dynamodbav:"name"           json:"name"`
	Strain         string     `dynamodbav:"strain"         json:"strain"`
	Genetics       string     `dynamodbav:"genetics"       json:"genetics,omitempty"`
	SeedBank       string     `dynamodbav:"seedBank"       json:"seedBank,omitempty"`
	PlantType      PlantType  `dynamodbav:"plantType"      json:"plantType,omitempty"`
	Phase          PlantPhase `dynamodbav:"phase"          json:"phase"`
	PhaseStartDate string     `dynamodbav:"phaseStartDate" json:"phaseStartDate"`
	AvatarKey     string `dynamodbav:"avatarKey"     json:"avatarKey,omitempty"`
	EnvironmentID string `dynamodbav:"environmentId" json:"environmentId,omitempty"`
	ArchivedAt    string `dynamodbav:"archivedAt"    json:"archivedAt,omitempty"`
	CreatedAt     string `dynamodbav:"createdAt"     json:"createdAt"`
}

type CreatePlantRequest struct {
	Name          string     `json:"name"`
	Strain        string     `json:"strain"`
	Genetics      string     `json:"genetics,omitempty"`
	SeedBank      string     `json:"seedBank,omitempty"`
	PlantType     PlantType  `json:"plantType,omitempty"`
	Phase         PlantPhase `json:"phase"`
	EnvironmentID string     `json:"environmentId,omitempty"`
}

type UpdatePlantDetailsRequest struct {
	Name      string    `json:"name"`
	Strain    string    `json:"strain"`
	Genetics  string    `json:"genetics,omitempty"`
	SeedBank  string    `json:"seedBank,omitempty"`
	PlantType PlantType `json:"plantType,omitempty"`
}

type UpdatePlantRequest struct {
	Name          *string     `json:"name,omitempty"`
	Strain        *string     `json:"strain,omitempty"`
	Genetics      *string     `json:"genetics,omitempty"`
	SeedBank      *string     `json:"seedBank,omitempty"`
	Phase         *PlantPhase `json:"phase,omitempty"`
	AvatarKey     *string     `json:"avatarKey,omitempty"`
	EnvironmentID *string     `json:"environmentId,omitempty"`
}

package model

type Settings struct {
	UserID            string    `dynamodbav:"userId"                      json:"userId"`
	ShortcutLogTypes  []LogType `dynamodbav:"shortcutLogTypes,omitempty"  json:"shortcutLogTypes,omitempty"`
	SortChipOrder     []LogType `dynamodbav:"sortChipOrder,omitempty"     json:"sortChipOrder,omitempty"`
	PlantsLayoutMode  string    `dynamodbav:"plantsLayoutMode,omitempty"  json:"plantsLayoutMode,omitempty"`
	NutrientEntryMode string    `dynamodbav:"nutrientEntryMode,omitempty" json:"nutrientEntryMode,omitempty"`
}

type UpdateSettingsRequest struct {
	ShortcutLogTypes  []LogType `json:"shortcutLogTypes,omitempty"`
	SortChipOrder     []LogType `json:"sortChipOrder,omitempty"`
	PlantsLayoutMode  string    `json:"plantsLayoutMode,omitempty"`
	NutrientEntryMode string    `json:"nutrientEntryMode,omitempty"`
}

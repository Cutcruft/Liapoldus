package domain

import "time"

type Form struct {
	ID         string         `json:"id"`
	SiteID     string         `json:"siteId"`
	Name       string         `json:"name"`
	Definition map[string]any `json:"definition"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

type Submission struct {
	ID        string         `json:"id"`
	SiteID    string         `json:"siteId"`
	FormID    string         `json:"formId"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"createdAt"`
}

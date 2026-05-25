package types

import "time"

type Organization struct {
	ID                 int64                  `json:"id"`
	URL                string                 `json:"url,omitempty"`
	Name               string                 `json:"name"`
	Details            string                 `json:"details,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	Tags               []string               `json:"tags,omitempty"`
	OrganizationFields map[string]interface{} `json:"organization_fields,omitempty"`
	CreatedAt          time.Time              `json:"created_at,omitempty"`
	UpdatedAt          time.Time              `json:"updated_at,omitempty"`
}

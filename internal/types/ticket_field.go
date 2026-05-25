package types

import "time"

type TicketField struct {
	ID                 int64               `json:"id"`
	URL                string              `json:"url,omitempty"`
	Type               string              `json:"type"`
	Title              string              `json:"title"`
	RawTitle           string              `json:"raw_title,omitempty"`
	Description        string              `json:"description,omitempty"`
	Active             bool                `json:"active"`
	Position           int                 `json:"position,omitempty"`
	AgentDescription   string              `json:"agent_description,omitempty"`
	CustomFieldOptions []TicketFieldOption `json:"custom_field_options,omitempty"`
	CreatedAt          time.Time           `json:"created_at,omitempty"`
	UpdatedAt          time.Time           `json:"updated_at,omitempty"`
}

type TicketFieldOption struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	RawName   string    `json:"raw_name,omitempty"`
	Value     string    `json:"value"`
	Default   bool      `json:"default,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type TicketFieldPage struct {
	TicketFields []TicketField `json:"ticket_fields"`
	Meta         PageMeta      `json:"meta"`
	Links        PageLinks     `json:"links"`
	Count        int           `json:"count,omitempty"`
}

type ListTicketFieldsOptions struct {
	Limit  int
	Cursor string
}

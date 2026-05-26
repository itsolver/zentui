package types

import (
	"encoding/json"
	"time"
)

// AuditEvent represents a single event within a ticket audit.
type AuditEvent struct {
	ID            int64        `json:"id"`
	Type          string       `json:"type"`
	FieldName     string       `json:"field_name,omitempty"`
	Value         interface{}  `json:"value"`
	PreviousValue interface{}  `json:"previous_value"`
	Body          string       `json:"body,omitempty"`
	HTMLBody      string       `json:"html_body,omitempty"`
	Public        *bool        `json:"public,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	AuthorID      int64        `json:"author_id,omitempty"`
}

// UnmarshalJSON accepts Zendesk audit events whose comment body fields are not
// always strings. Some system/app audit events return structured JSON objects.
func (e *AuditEvent) UnmarshalJSON(data []byte) error {
	type alias AuditEvent
	var raw struct {
		alias
		Body     json.RawMessage `json:"body,omitempty"`
		HTMLBody json.RawMessage `json:"html_body,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = AuditEvent(raw.alias)
	var err error
	e.Body, err = auditBodyToString(raw.Body)
	if err != nil {
		return err
	}
	e.HTMLBody, err = auditBodyToString(raw.HTMLBody)
	return err
}

func auditBodyToString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, nil
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Audit represents a single audit entry for a ticket.
type Audit struct {
	ID        int64        `json:"id"`
	TicketID  int64        `json:"ticket_id"`
	AuthorID  int64        `json:"author_id"`
	CreatedAt time.Time    `json:"created_at"`
	Events    []AuditEvent `json:"events"`
}

// AuditPage represents a paginated response of audits.
type AuditPage struct {
	Audits []Audit   `json:"audits"`
	Users  []User    `json:"users,omitempty"`
	Meta   PageMeta  `json:"meta"`
	Links  PageLinks `json:"links"`
	Count  int       `json:"count,omitempty"`
}

// ListAuditsOptions configures the ListAudits request.
type ListAuditsOptions struct {
	Limit     int
	Cursor    string
	SortOrder string
	Include   string
}

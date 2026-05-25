package types

import "time"

type UserIdentity struct {
	ID                 int64      `json:"id"`
	URL                string     `json:"url,omitempty"`
	UserID             int64      `json:"user_id"`
	Type               string     `json:"type"`
	Value              string     `json:"value"`
	Primary            bool       `json:"primary"`
	Verified           bool       `json:"verified"`
	VerificationMethod string     `json:"verification_method,omitempty"`
	DeliverableState   string     `json:"deliverable_state,omitempty"`
	CreatedAt          time.Time  `json:"created_at,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at,omitempty"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
}

type UserIdentityPage struct {
	Identities []UserIdentity `json:"identities"`
	Meta       PageMeta       `json:"meta"`
	Links      PageLinks      `json:"links"`
	Count      int            `json:"count,omitempty"`
}

type ListUserIdentitiesOptions struct {
	Limit  int
	Cursor string
	Types  []string
}

type CreateUserIdentityRequest struct {
	Type               string `json:"type"`
	Value              string `json:"value"`
	Verified           *bool  `json:"verified,omitempty"`
	VerificationMethod string `json:"verification_method,omitempty"`
	Primary            *bool  `json:"primary,omitempty"`
}

type JobStatus struct {
	ID       string                 `json:"id"`
	Message  string                 `json:"message,omitempty"`
	Status   string                 `json:"status,omitempty"`
	Progress int                    `json:"progress,omitempty"`
	Total    int                    `json:"total,omitempty"`
	URL      string                 `json:"url,omitempty"`
	Results  []JobStatusResult      `json:"results,omitempty"`
	Raw      map[string]interface{} `json:"-"`
}

type JobStatusResult struct {
	Action  string `json:"action,omitempty"`
	ID      int64  `json:"id,omitempty"`
	Status  string `json:"status,omitempty"`
	Success bool   `json:"success,omitempty"`
}

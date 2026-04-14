package types

type User struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	RoleType        *int   `json:"role_type"`
	RestrictedAgent bool   `json:"restricted_agent"`
	CustomRoleID    *int64 `json:"custom_role_id"`
	Active          bool   `json:"active"`
}

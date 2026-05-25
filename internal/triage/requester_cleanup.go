package triage

import (
	"context"
	"regexp"
	"strings"

	"github.com/itsolver/zentui/internal/types"
	"github.com/itsolver/zentui/pkg/zendesk"
)

var phoneTextRe = regexp.MustCompile(`(?:\+|00)?\d[\d\s().-]{6,}\d`)

type UserSummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type RequesterCleanupPlan struct {
	Eligible       bool        `json:"eligible"`
	DefaultEnabled bool        `json:"default_enabled"`
	Reason         string      `json:"reason,omitempty"`
	PhoneNumber    string      `json:"phone_number,omitempty"`
	SourceUser     UserSummary `json:"source_user,omitempty"`
	TargetUser     UserSummary `json:"target_user,omitempty"`
}

type RequesterCleanupResult struct {
	Requested      bool        `json:"requested"`
	Performed      bool        `json:"performed"`
	Status         string      `json:"status"`
	Reason         string      `json:"reason,omitempty"`
	PhoneNumber    string      `json:"phone_number,omitempty"`
	SourceUser     UserSummary `json:"source_user,omitempty"`
	TargetUser     UserSummary `json:"target_user,omitempty"`
	MergeStatus    string      `json:"merge_status,omitempty"`
	IdentityStatus string      `json:"identity_status,omitempty"`
}

func BuildRequesterCleanupPlan(source types.Ticket, sourceAudits []types.Audit, sourceUser *types.User, target types.Ticket, targetUser *types.User) RequesterCleanupPlan {
	if !IsZoomPhoneTicket(source) {
		return RequesterCleanupPlan{Eligible: false, Reason: "source_not_zoom_phone_ticket", SourceUser: summarizeUser(sourceUser)}
	}
	if source.RequesterID == 0 {
		return RequesterCleanupPlan{Eligible: false, Reason: "source_requester_missing"}
	}
	phone, reason := SourceTicketPhoneNumber(source, sourceAudits)
	if phone == "" {
		return RequesterCleanupPlan{Eligible: false, Reason: reason, SourceUser: summarizeUser(sourceUser)}
	}
	if target.RequesterID == 0 {
		return RequesterCleanupPlan{Eligible: false, Reason: "target_requester_missing", PhoneNumber: phone, SourceUser: summarizeUser(sourceUser)}
	}
	if source.RequesterID == target.RequesterID {
		return RequesterCleanupPlan{Eligible: false, Reason: "same_requester", PhoneNumber: phone, SourceUser: summarizeUser(sourceUser), TargetUser: summarizeUser(targetUser)}
	}
	return RequesterCleanupPlan{
		Eligible:       true,
		DefaultEnabled: true,
		PhoneNumber:    phone,
		SourceUser:     summarizeUser(sourceUser),
		TargetUser:     summarizeUser(targetUser),
	}
}

func ExecuteRequesterCleanup(ctx context.Context, users zendesk.UserService, plan RequesterCleanupPlan) (RequesterCleanupResult, error) {
	result := RequesterCleanupResult{
		Requested:   true,
		Performed:   false,
		Status:      "skipped",
		Reason:      plan.Reason,
		PhoneNumber: plan.PhoneNumber,
		SourceUser:  plan.SourceUser,
		TargetUser:  plan.TargetUser,
	}
	if !plan.Eligible {
		if result.Reason == "" {
			result.Reason = "requester_cleanup_unavailable"
		}
		return result, nil
	}
	if _, err := users.MergeEndUser(ctx, plan.SourceUser.ID, plan.TargetUser.ID); err != nil {
		result.Status = "failed"
		result.Reason = "requester_merge_failed"
		return result, err
	}
	result.Performed = true
	result.MergeStatus = "complete"

	identityStatus, err := ensurePhoneIdentity(ctx, users, plan.TargetUser.ID, plan.PhoneNumber)
	if err != nil {
		result.Status = "failed"
		result.Reason = "phone_identity_failed"
		return result, err
	}
	result.Status = "complete"
	result.IdentityStatus = identityStatus
	return result, nil
}

func ensurePhoneIdentity(ctx context.Context, users zendesk.UserService, userID int64, phone string) (string, error) {
	page, err := users.ListIdentities(ctx, userID, &types.ListUserIdentitiesOptions{Types: []string{"phone_number", "phone"}})
	if err != nil {
		return "", err
	}
	for _, identity := range page.Identities {
		if PhoneIdentityMatches(identity, phone) {
			return "already_present", nil
		}
	}
	verified := true
	if _, err := users.CreateIdentity(ctx, userID, &types.CreateUserIdentityRequest{Type: "phone_number", Value: phone, Verified: &verified}); err != nil {
		return "", err
	}
	return "created", nil
}

func IsZoomPhoneTicket(ticket types.Ticket) bool {
	for _, tag := range ticket.Tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "phone_call", "zoom_phone", "phone_transcript":
			return true
		}
	}
	subject := strings.ToLower(strings.TrimSpace(ticket.Subject))
	return strings.HasPrefix(subject, "phone call") || strings.HasPrefix(subject, "incoming call") || strings.HasPrefix(subject, "outbound call")
}

func SourceTicketPhoneNumber(ticket types.Ticket, audits []types.Audit) (string, string) {
	primary := strings.Join([]string{ticket.Subject, ticket.Description}, "\n")
	primaryNumbers := ExtractNormalizedPhoneNumbers(primary)
	if len(primaryNumbers) == 1 {
		return primaryNumbers[0], ""
	}
	if len(primaryNumbers) > 1 {
		return "", "ambiguous_phone_number"
	}

	var commentText strings.Builder
	for _, audit := range audits {
		for _, event := range audit.Events {
			if event.Type == "Comment" {
				commentText.WriteString(event.Body)
				commentText.WriteByte('\n')
				commentText.WriteString(event.HTMLBody)
				commentText.WriteByte('\n')
			}
		}
	}
	commentNumbers := ExtractNormalizedPhoneNumbers(commentText.String())
	if len(commentNumbers) == 1 {
		return commentNumbers[0], ""
	}
	if len(commentNumbers) > 1 {
		return "", "ambiguous_phone_number"
	}
	return "", "missing_phone_number"
}

func ExtractNormalizedPhoneNumbers(text string) []string {
	var numbers []string
	seen := map[string]bool{}
	for _, match := range phoneTextRe.FindAllString(text, -1) {
		normalized := NormalizePhoneToE164(match)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		numbers = append(numbers, normalized)
	}
	return numbers
}

func NormalizePhoneToE164(value string) string {
	cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(strings.TrimSpace(value), "")
	if cleaned == "" {
		return ""
	}
	if strings.HasPrefix(cleaned, "00") {
		cleaned = "+" + cleaned[2:]
	}
	if strings.HasPrefix(cleaned, "+") {
		digits := regexp.MustCompile(`\D`).ReplaceAllString(cleaned, "")
		if len(digits) >= 8 && len(digits) <= 15 {
			return "+" + digits
		}
		return ""
	}
	digits := regexp.MustCompile(`\D`).ReplaceAllString(cleaned, "")
	if strings.HasPrefix(digits, "0") && len(digits) >= 9 {
		return "+61" + digits[1:]
	}
	if strings.HasPrefix(digits, "61") && len(digits) >= 10 {
		return "+" + digits
	}
	return ""
}

func PhoneIdentityMatches(identity types.UserIdentity, phone string) bool {
	typ := strings.ToLower(strings.TrimSpace(identity.Type))
	if typ != "phone_number" && typ != "phone" {
		return false
	}
	return NormalizePhoneToE164(identity.Value) == phone
}

func summarizeUser(user *types.User) UserSummary {
	if user == nil {
		return UserSummary{}
	}
	return UserSummary{ID: user.ID, Name: user.Name, Email: user.Email}
}

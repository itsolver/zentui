package tui

import (
	"fmt"
	"strings"

	"github.com/itsolver/zentui/internal/types"
)

// TimelineNode represents one visual node (one audit, possibly with grouped events).
type TimelineNode struct {
	Audit    types.Audit
	Comments []types.AuditEvent
	Changes  []types.AuditEvent
}

// relevantFieldNames is the set of field changes to show in the timeline.
var relevantFieldNames = map[string]bool{
	"status":      true,
	"priority":    true,
	"assignee_id": true,
	"group_id":    true,
	"subject":     true,
	"tags":        true,
}

// buildTimeline converts audits into renderable nodes, filtering irrelevant events.
func buildTimeline(audits []types.Audit) []TimelineNode {
	var nodes []TimelineNode
	for _, audit := range audits {
		var comments []types.AuditEvent
		var changes []types.AuditEvent

		for _, ev := range audit.Events {
			switch ev.Type {
			case "Comment":
				if ev.Body != "" {
					comments = append(comments, ev)
				}
			case "Change", "Create":
				if relevantFieldNames[ev.FieldName] {
					changes = append(changes, ev)
				}
			}
		}

		if len(comments) > 0 || len(changes) > 0 {
			nodes = append(nodes, TimelineNode{
				Audit:    audit,
				Comments: comments,
				Changes:  changes,
			})
		}
	}
	return nodes
}

// filterCommentNodes returns only nodes that contain comment events.
func filterCommentNodes(nodes []TimelineNode) []TimelineNode {
	var filtered []TimelineNode
	for _, n := range nodes {
		if len(n.Comments) > 0 {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

// renderTimeline renders the vertical timeline string for the viewport.
func renderTimeline(nodes []TimelineNode, users map[int64]types.User, width int) string {
	if len(nodes) == 0 {
		return ""
	}

	bodyWidth := width - 5 // gutter: " │  " = 4 chars + 1 padding
	if bodyWidth < 20 {
		bodyWidth = 20
	}

	var b strings.Builder
	connector := timelineConnectorStyle.Render

	for i, node := range nodes {
		isLast := i == len(nodes)-1

		// Determine node icon and author line
		timeStr := relativeTime(node.Audit.CreatedAt)
		author := timelineUserName(node.Audit.AuthorID, users)

		icon := nodeIcon(node)
		branch := "├─"
		if isLast {
			branch = "╰─"
		}

		// Header: branch + icon + time + author
		b.WriteString(connector(" "+branch+" ") + icon + " " +
			commentTimeStyle.Render(timeStr) +
			" " + connector("·") + " " +
			commentAuthorStyle.Render(author) + "\n")

		// Render change events
		for _, ch := range node.Changes {
			line := renderFieldChange(ch, users)
			b.WriteString(connector(" │  ") + line + "\n")
		}

		// Render comment events
		for _, c := range node.Comments {
			isPublic := c.Public == nil || *c.Public

			if !isPublic {
				b.WriteString(connector(" │  ") + internalNoteStyle.Render("(internal)") + "\n")
			}

			// Render body with markdown if HTML available
			rendered := renderMarkdown(c.HTMLBody, c.Body, bodyWidth)
			for _, line := range strings.Split(rendered, "\n") {
				b.WriteString(connector(" │  ") + line + "\n")
			}

			// Attachments
			for _, a := range c.Attachments {
				icon := "📎"
				style := attachmentStyle
				if a.IsImage() {
					icon = "📷"
					style = attachmentImageStyle
				}
				b.WriteString(connector(" │  ") + "  " +
					style.Render(fmt.Sprintf("%s %s (%s)", icon, a.FileName, a.HumanSize())) + "\n")
			}
		}

		// Blank line between nodes (connector continues)
		if !isLast {
			b.WriteString(connector(" │") + "\n")
		}
	}

	return b.String()
}

func renderConversation(comments []types.Comment, ticket types.Ticket, users map[int64]types.User, width int) string {
	if len(comments) == 0 {
		return ""
	}

	if width < 40 {
		width = 40
	}
	bodyWidth := width - 4
	if bodyWidth < 20 {
		bodyWidth = 20
	}

	var b strings.Builder
	for i, comment := range comments {
		if i > 0 {
			b.WriteString("\n")
		}

		author := timelineUserName(comment.AuthorID, users)
		headerParts := []string{commentAuthorStyle.Render(author)}
		if channel := commentChannel(comment); channel != "" {
			headerParts = append(headerParts, commentTimeStyle.Render(channel))
		}
		if !comment.CreatedAt.IsZero() {
			headerParts = append(headerParts, commentTimeStyle.Render(relativeTime(comment.CreatedAt)))
		}
		if !commentIsPublic(comment) {
			headerParts = append(headerParts, internalNoteStyle.Render("internal note"))
		}
		b.WriteString(strings.Join(headerParts, "  ·  ") + "\n")

		if recipients := commentRecipients(comment, ticket, users); recipients != "" {
			b.WriteString(commentAuthorStyle.Render("To:") + " " + valueStyle.Render(recipients) + "\n")
		}

		body := commentPlainBody(comment)
		rendered := renderMarkdown(comment.HTMLBody, body, bodyWidth)
		rendered = strings.TrimSpace(rendered)
		if rendered == "" {
			rendered = dimStyle.Render("(empty comment)")
		}

		cardStyle := conversationCustomerStyle
		if commentIsAgent(comment, ticket, users) {
			cardStyle = conversationAgentStyle
		}
		if !commentIsPublic(comment) {
			cardStyle = conversationInternalStyle
		}
		b.WriteString(cardStyle.Width(width-2).Render(rendered) + "\n")

		for _, a := range comment.Attachments {
			icon := "📎"
			style := attachmentStyle
			if a.IsImage() {
				icon = "📷"
				style = attachmentImageStyle
			}
			b.WriteString("  " + style.Render(fmt.Sprintf("%s %s (%s)", icon, a.FileName, a.HumanSize())) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func commentPlainBody(comment types.Comment) string {
	if comment.PlainBody != "" {
		return comment.PlainBody
	}
	return comment.Body
}

func commentIsPublic(comment types.Comment) bool {
	return comment.Public == nil || *comment.Public
}

func commentIsAgent(comment types.Comment, ticket types.Ticket, users map[int64]types.User) bool {
	if u, ok := users[comment.AuthorID]; ok {
		return u.Role != "" && u.Role != "end-user"
	}
	return comment.AuthorID != 0 && comment.AuthorID != ticket.RequesterID
}

func commentChannel(comment types.Comment) string {
	via := commentVia(comment)
	if via == nil || via.Channel == nil {
		return ""
	}
	channel := fmt.Sprint(via.Channel)
	channel = strings.Trim(channel, `" `)
	channel = strings.ReplaceAll(channel, "_", " ")
	if channel == "" || channel == "<nil>" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(channel), "via ") {
		return channel
	}
	return "via " + channel
}

func commentVia(comment types.Comment) *types.CommentVia {
	if comment.Metadata != nil && comment.Metadata.Via != nil {
		return comment.Metadata.Via
	}
	return comment.Via
}

func commentRecipients(comment types.Comment, ticket types.Ticket, users map[int64]types.User) string {
	if via := commentVia(comment); via != nil {
		if recipients := formatCommentParties(via.Source.To); recipients != "" {
			return recipients
		}
	}

	if commentIsAgent(comment, ticket, users) {
		return timelineUserName(ticket.RequesterID, users)
	}
	if ticket.AssigneeID != 0 {
		return timelineUserName(ticket.AssigneeID, users)
	}
	return "Support"
}

func formatCommentParties(parties []types.CommentViaParty) string {
	if len(parties) == 0 {
		return ""
	}
	values := make([]string, 0, len(parties))
	for _, party := range parties {
		name := strings.TrimSpace(party.Name)
		address := strings.TrimSpace(party.Address)
		if address == "" {
			address = strings.TrimSpace(party.Email)
		}
		if name == "" {
			name = strings.TrimSpace(party.Title)
		}
		switch {
		case name != "" && address != "":
			values = append(values, fmt.Sprintf("%s <%s>", name, address))
		case name != "":
			values = append(values, name)
		case address != "":
			values = append(values, address)
		}
	}
	return strings.Join(values, ", ")
}

// nodeIcon returns the icon for a timeline node based on its content.
func nodeIcon(node TimelineNode) string {
	// If there are status changes, use the target status icon
	for _, ch := range node.Changes {
		if ch.FieldName == "status" {
			if val, ok := ch.Value.(string); ok {
				return styledStatus(val)[:len(statusIcons[val])+len(val)+1] // just get the icon
			}
		}
	}

	// For changes-only nodes (no comments), use a bullet
	if len(node.Comments) == 0 {
		return timelineChangeStyle.Render("●")
	}

	// For comment nodes
	return commentAuthorStyle.Render("●")
}

// renderFieldChange renders a single field change line.
func renderFieldChange(ev types.AuditEvent, users map[int64]types.User) string {
	arrow := timelineArrowStyle.Render(" → ")
	label := fieldLabel(ev.FieldName)
	prev := formatFieldValue(ev.FieldName, ev.PreviousValue, users)
	next := formatFieldValue(ev.FieldName, ev.Value, users)

	return timelineChangeStyle.Render(label+": ") + prev + arrow + next
}

// fieldLabel returns a human-readable label for a field name.
func fieldLabel(name string) string {
	switch name {
	case "status":
		return "Status"
	case "priority":
		return "Priority"
	case "assignee_id":
		return "Assignee"
	case "group_id":
		return "Group"
	case "subject":
		return "Subject"
	case "tags":
		return "Tags"
	default:
		return name
	}
}

// formatFieldValue formats a field value with appropriate styling.
func formatFieldValue(field string, val interface{}, users map[int64]types.User) string {
	s := fmt.Sprintf("%v", val)
	if s == "" || s == "<nil>" {
		return dimStyle.Render("none")
	}

	switch field {
	case "status":
		return styledStatus(s)
	case "priority":
		return styledPriority(s)
	case "assignee_id":
		return resolveUserValue(s, users)
	default:
		return timelineChangeStyle.Render(s)
	}
}

// resolveUserValue tries to resolve a user ID string to a name.
func resolveUserValue(val string, users map[int64]types.User) string {
	// Try parsing as int64
	var id int64
	if _, err := fmt.Sscanf(val, "%d", &id); err == nil && id > 0 {
		if u, ok := users[id]; ok {
			return commentAuthorStyle.Render(u.Name)
		}
		return dimStyle.Render(fmt.Sprintf("User #%d", id))
	}
	if val == "" || val == "0" {
		return dimStyle.Render("unassigned")
	}
	return timelineChangeStyle.Render(val)
}

// timelineUserName returns a user's name for the timeline header.
func timelineUserName(id int64, users map[int64]types.User) string {
	if id == 0 {
		return "System"
	}
	if u, ok := users[id]; ok {
		return u.Name
	}
	return fmt.Sprintf("User #%d", id)
}

// wrapText wraps text to the given width, preserving existing line breaks.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			result = append(result, "")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}

		line := words[0]
		if len(line) > width {
			for len(line) > width {
				result = append(result, line[:width])
				line = line[width:]
			}
		}
		for _, w := range words[1:] {
			if len(line)+1+len(w) > width {
				result = append(result, line)
				line = w
				// Break long words that exceed width
				for len(line) > width {
					result = append(result, line[:width])
					line = line[width:]
				}
			} else {
				line += " " + w
			}
		}
		result = append(result, line)
	}
	return result
}

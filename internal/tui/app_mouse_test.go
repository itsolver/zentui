package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/itsolver/zentui/internal/triage"
	"github.com/itsolver/zentui/internal/types"
)

func TestAppViewEnablesMouseModeOnNormalOverlayAndCommandPalette(t *testing.T) {
	app := mouseTestApp(t)

	assert.Equal(t, tea.MouseModeCellMotion, app.View().MouseMode)

	var cmd tea.Cmd
	app.actions, cmd = app.actions.openComment(1, app.perms)
	require.NotNil(t, cmd)
	assert.Equal(t, tea.MouseModeCellMotion, app.View().MouseMode)

	app.actions = app.actions.close()
	app.cmdPalette.active = true
	assert.Equal(t, tea.MouseModeCellMotion, app.View().MouseMode)
}

func TestMouseClickQueueRowSelectsAndLoadsTicket(t *testing.T) {
	app := mouseTestApp(t)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: 3, Y: 5, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Equal(t, 1, updated.list.cursor)
	assert.Equal(t, focusList, updated.focus)
	assert.Equal(t, int64(2), updated.detail.expectedID)
	assert.NotNil(t, cmd)
}

func TestMouseClickFieldOpensEditModalWithCurrentValue(t *testing.T) {
	app := mouseTestApp(t)
	fieldRegion := requireRegion(t, app.hitRegions(), hitFieldEdit)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: fieldRegion.X1, Y: fieldRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Equal(t, actionField, updated.actions.mode)
	assert.Equal(t, int64(99), updated.actions.fieldID)
	assert.Equal(t, "Support Plan", updated.actions.fieldLabel)
	assert.Equal(t, "Managed", updated.actions.textarea.Value())
	assert.NotNil(t, cmd)
}

func TestSubmitFieldEditWritesOnlySelectedCustomField(t *testing.T) {
	svc := &mouseTicketService{}
	model := newActionsModel(svc, nil)
	var cmd tea.Cmd
	model, cmd = model.openField(123, 99, "Support Plan", "Managed", "text")
	require.NotNil(t, cmd)
	model.textarea.SetValue("Ad hoc")

	msg := model.submitField()()

	_, ok := msg.(ticketUpdatedMsg)
	require.True(t, ok, "expected ticketUpdatedMsg, got %T", msg)
	require.NotNil(t, svc.lastUpdate)
	require.Len(t, svc.lastUpdate.CustomFields, 1)
	assert.Equal(t, int64(99), svc.lastUpdate.CustomFields[0].ID)
	assert.Equal(t, "Ad hoc", svc.lastUpdate.CustomFields[0].Value)
	assert.Nil(t, svc.lastUpdate.Comment)
	assert.Empty(t, svc.lastUpdate.Status)
}

func TestInvalidFieldUpdateKeepsModalAndTypedValue(t *testing.T) {
	svc := &mouseTicketService{updateErr: errors.New("invalid custom field")}
	model := newActionsModel(svc, nil)
	model, _ = model.openField(123, 99, "Support Plan", "Managed", "text")
	model.textarea.SetValue("Bad value")

	msg := model.submitField()()
	updated, cmd := model.Update(msg)

	assert.Nil(t, cmd)
	assert.Equal(t, actionField, updated.mode)
	assert.Equal(t, "Bad value", updated.textarea.Value())
	require.NotNil(t, updated.err)
	assert.Contains(t, updated.err.Error(), "invalid custom field")
}

func TestMouseClickAssetOpensLocalPath(t *testing.T) {
	app := mouseTestApp(t)
	var opened string
	app.openPath = func(path string) { opened = path }

	assetRegion := requireRegion(t, app.hitRegions(), hitAssetFile)
	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: assetRegion.X1, Y: assetRegion.Y1, Button: tea.MouseLeft}))

	assert.Nil(t, cmd)
	assert.Equal(t, app.operator.assets[0].LocalPath, opened)
	assert.IsType(t, App{}, model)
}

func TestMouseClickCommandBarDraftUsesDraftPath(t *testing.T) {
	app := mouseTestApp(t)
	commandRegion := requireCommandRegion(t, app.commandHitRegions(), "draft")

	model, _ := app.Update(tea.MouseClickMsg(tea.Mouse{X: commandRegion.X1, Y: commandRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.True(t, updated.draftBusy)
}

func TestMouseWheelAffectsOnlyPaneUnderPointer(t *testing.T) {
	app := mouseTestApp(t)
	app.list.cursor = 1

	model, _ := app.Update(tea.MouseWheelMsg(tea.Mouse{X: app.listPanelWidth() + 5, Y: 8, Button: tea.MouseWheelDown}))
	updated := model.(App)

	assert.Equal(t, 1, updated.list.cursor)

	model, _ = updated.Update(tea.MouseWheelMsg(tea.Mouse{X: 3, Y: 8, Button: tea.MouseWheelUp}))
	updated = model.(App)

	assert.Equal(t, 0, updated.list.cursor)
}

func TestMouseClickMergeSubmitKeepsExplicitPreviewStep(t *testing.T) {
	app := mouseTestApp(t)
	app.actions, _ = app.actions.openMerge(1, nil, 0)
	app.actions.textarea.SetValue("2")
	submitRegion := requireRegion(t, app.actionHitRegions(), hitActionSubmit)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: submitRegion.X1, Y: submitRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Equal(t, actionMerge, updated.actions.mode)
	assert.True(t, updated.actions.submitting)
	assert.False(t, updated.actions.mergePreviewReady)
	assert.NotNil(t, cmd)
}

func mouseTestApp(t *testing.T) App {
	t.Helper()
	svc := &mouseTicketService{}
	app := NewAppWithOptions(svc, nil, nil, "example", "test", AppOptions{WorkDir: t.TempDir()})
	app.width = 160
	app.height = 40
	app.state = splitView
	app.showDetail = true
	app.focus = focusList
	app.list.width = app.listPanelWidth()
	app.list.height = app.height
	now := time.Now()
	app.list.items = []types.Ticket{
		{ID: 1, Subject: "First", Status: "open", UpdatedAt: now, CreatedAt: now},
		{ID: 2, Subject: "Second", Status: "pending", UpdatedAt: now, CreatedAt: now},
	}
	app.list.loading = false
	app.operator.setSize(app.operatorPanelWidth(), app.height)
	app.operator.setTicketFields([]types.TicketField{{ID: 99, Type: "text", Title: "Support Plan"}})
	app.operator.setTicket(
		types.Ticket{ID: 1, RequesterID: 10, OrganizationID: 20, CustomFields: []types.CustomField{{ID: 99, Value: "Managed"}}},
		[]types.User{{ID: 10, Name: "Alice", Email: "alice@example.com"}},
		[]types.Organization{{ID: 20, Name: "Acme"}},
		1,
	)
	app.operator.setAssets(triage.Manifest{TicketID: 1, Assets: []triage.AssetRecord{{Filename: "screenshot.png", LocalPath: "/tmp/screenshot.png", SHA256: "abc"}}}, nil)
	return app
}

func requireRegion(t *testing.T, regions []hitRegion, action hitAction) hitRegion {
	t.Helper()
	for _, region := range regions {
		if region.Action == action {
			return region
		}
	}
	t.Fatalf("missing hit region %s in %#v", action, regions)
	return hitRegion{}
}

func requireCommandRegion(t *testing.T, regions []hitRegion, command string) hitRegion {
	t.Helper()
	for _, region := range regions {
		if region.Action == hitCommand && region.Command == command {
			return region
		}
	}
	t.Fatalf("missing command hit region %s in %#v", command, regions)
	return hitRegion{}
}

type mouseTicketService struct {
	lastUpdate *types.UpdateTicketRequest
	updateErr  error
}

func (s *mouseTicketService) List(context.Context, *types.ListTicketsOptions) (*types.TicketPage, error) {
	return &types.TicketPage{}, nil
}

func (s *mouseTicketService) ListView(context.Context, int64, *types.ListTicketsOptions) (*types.TicketPage, error) {
	return &types.TicketPage{}, nil
}

func (s *mouseTicketService) Get(_ context.Context, id int64, _ *types.GetTicketOptions) (*types.TicketResult, error) {
	return &types.TicketResult{Ticket: types.Ticket{ID: id, Subject: "Ticket", Status: "open"}}, nil
}

func (s *mouseTicketService) Create(context.Context, *types.CreateTicketRequest) (*types.Ticket, error) {
	return nil, nil
}

func (s *mouseTicketService) Update(_ context.Context, id int64, req *types.UpdateTicketRequest) (*types.Ticket, error) {
	s.lastUpdate = req
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return &types.Ticket{ID: id}, nil
}

func (s *mouseTicketService) Delete(context.Context, int64) error { return nil }

func (s *mouseTicketService) ListComments(context.Context, int64, *types.ListCommentsOptions) (*types.CommentPage, error) {
	return &types.CommentPage{}, nil
}

func (s *mouseTicketService) ListAudits(context.Context, int64, *types.ListAuditsOptions) (*types.AuditPage, error) {
	return &types.AuditPage{}, nil
}

func (s *mouseTicketService) ListTicketFields(context.Context, *types.ListTicketFieldsOptions) (*types.TicketFieldPage, error) {
	return &types.TicketFieldPage{}, nil
}

func (s *mouseTicketService) MergeTickets(context.Context, int64, *types.MergeTicketsRequest) (*types.MergeTicketsResult, error) {
	return &types.MergeTicketsResult{}, nil
}

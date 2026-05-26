package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestMouseReleaseQueueRowSelectsAndLoadsTicket(t *testing.T) {
	app := mouseTestApp(t)

	model, cmd := app.Update(tea.MouseReleaseMsg(tea.Mouse{X: 3, Y: 5, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Equal(t, 1, updated.list.cursor)
	assert.Equal(t, focusList, updated.focus)
	assert.Equal(t, int64(2), updated.detail.expectedID)
	assert.NotNil(t, cmd)
}

func TestMouseClickLastQueueRowUsesListCursorSideEffects(t *testing.T) {
	app := mouseTestApp(t)
	app.list.hasMore = true
	app.list.afterCursor = "next"
	lastRegion := requireQueueRegion(t, app.hitRegions(), len(app.list.items)-1)

	model, _ := app.Update(tea.MouseClickMsg(tea.Mouse{X: lastRegion.X1, Y: lastRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Equal(t, len(app.list.items)-1, updated.list.cursor)
	assert.True(t, updated.list.loadingMore)
}

func TestMouseClickFieldStartsInlineEditWithCurrentValue(t *testing.T) {
	app := mouseTestApp(t)
	fieldRegion := requireRegion(t, app.hitRegions(), hitFieldEdit)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: fieldRegion.X1, Y: fieldRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Equal(t, actionNone, updated.actions.mode)
	assert.True(t, updated.operator.fieldEdit.active)
	assert.Equal(t, focusOperator, updated.focus)
	assert.Equal(t, int64(99), updated.operator.fieldEdit.fieldID)
	assert.Equal(t, "Support Plan", updated.operator.fieldEdit.label)
	assert.Equal(t, "Managed", updated.operator.fieldEdit.input.Value())
	assert.NotNil(t, cmd)
}

func TestCommandBarFieldDoesNotUseStaleOperatorTicket(t *testing.T) {
	app := mouseTestApp(t)
	app.list.cursor = 1
	commandRegion := requireCommandRegion(t, app.commandHitRegions(), "edit-field")

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: commandRegion.X1, Y: commandRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Nil(t, cmd)
	assert.Equal(t, actionNone, updated.actions.mode)
	assert.False(t, updated.operator.fieldEdit.active)
}

func TestSubmitInlineFieldEditWritesOnlySelectedCustomField(t *testing.T) {
	svc := &mouseTicketService{}
	app := mouseTestAppWithService(t, svc)
	fieldRegion := requireRegion(t, app.hitRegions(), hitFieldEdit)
	model, _ := app.Update(tea.MouseClickMsg(tea.Mouse{X: fieldRegion.X1, Y: fieldRegion.Y1, Button: tea.MouseLeft}))
	app = model.(App)
	app.operator.fieldEdit.input.SetValue("Ad hoc")

	model, cmd := app.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	app = model.(App)
	require.NotNil(t, cmd)
	require.True(t, app.operator.fieldEdit.submitting)

	model, _ = app.Update(cmd())
	updated := model.(App)

	require.NotNil(t, svc.lastUpdate)
	require.Len(t, svc.lastUpdate.CustomFields, 1)
	assert.Equal(t, int64(99), svc.lastUpdate.CustomFields[0].ID)
	assert.Equal(t, "Ad hoc", svc.lastUpdate.CustomFields[0].Value)
	assert.Nil(t, svc.lastUpdate.Comment)
	assert.Empty(t, svc.lastUpdate.Status)
	assert.False(t, updated.operator.fieldEdit.active)
}

func TestInvalidInlineFieldUpdateKeepsEditOpenWithTypedValue(t *testing.T) {
	svc := &mouseTicketService{updateErr: errors.New("invalid custom field")}
	app := mouseTestAppWithService(t, svc)
	fieldRegion := requireRegion(t, app.hitRegions(), hitFieldEdit)
	model, _ := app.Update(tea.MouseClickMsg(tea.Mouse{X: fieldRegion.X1, Y: fieldRegion.Y1, Button: tea.MouseLeft}))
	app = model.(App)
	app.operator.fieldEdit.input.SetValue("Bad value")

	model, cmd := app.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	app = model.(App)
	require.NotNil(t, cmd)

	model, _ = app.Update(cmd())
	updated := model.(App)

	assert.True(t, updated.operator.fieldEdit.active)
	assert.False(t, updated.operator.fieldEdit.submitting)
	assert.Equal(t, "Bad value", updated.operator.fieldEdit.input.Value())
	require.NotNil(t, updated.operator.fieldEdit.err)
	assert.Contains(t, updated.operator.fieldEdit.err.Error(), "invalid custom field")
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

func TestHitRegionsDoNotCreateTicketWorkFolder(t *testing.T) {
	app := mouseTestApp(t)
	ticketDir := filepath.Join(app.workCache.Root, "1")

	_ = app.hitRegions()

	_, err := os.Stat(ticketDir)
	assert.True(t, os.IsNotExist(err), "hit-testing should not create %s", ticketDir)
}

func TestMouseClickAssetsHeaderOpensTicketFolder(t *testing.T) {
	app := mouseTestApp(t)
	var opened string
	app.openPath = func(path string) { opened = path }

	folderRegion := requireRegion(t, app.hitRegions(), hitAssetsFolder)
	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: folderRegion.X1, Y: folderRegion.Y1, Button: tea.MouseLeft}))

	assert.Nil(t, cmd)
	assert.NotEmpty(t, opened)
	assert.Contains(t, opened, "1")
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

func TestOperatorFocusDoesNotRouteArrowsToQueue(t *testing.T) {
	app := mouseTestApp(t)
	operatorPane := requireRegion(t, app.paneHitRegions(), hitPaneOperator)
	model, _ := app.Update(tea.MouseClickMsg(tea.Mouse{X: operatorPane.X1, Y: operatorPane.Y1, Button: tea.MouseLeft}))
	updated := model.(App)
	require.Equal(t, focusOperator, updated.focus)

	model, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
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

func TestMouseClickMergeSuggestionSelectsTargetWithoutSubmitting(t *testing.T) {
	app := mouseTestApp(t)
	app.actions, _ = app.actions.openMerge(1, []triage.MergeSuggestion{
		{ID: 2, Subject: "First target", Status: "open"},
		{ID: 3, Subject: "Second target", Status: "pending"},
	}, 0)
	optionRegion := requireActionOptionRegion(t, app.actionHitRegions(), 1)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: optionRegion.X1, Y: optionRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Nil(t, cmd)
	assert.Equal(t, actionMerge, updated.actions.mode)
	assert.Equal(t, 1, updated.actions.mergeSelection)
	assert.Equal(t, "3", updated.actions.textarea.Value())
	assert.False(t, updated.actions.submitting)
}

func TestMouseClickOutsideExplicitModalButtonDoesNotSubmit(t *testing.T) {
	app := mouseTestApp(t)
	app.actions, _ = app.actions.openApproval(1, app.perms, "Reply", "pending", "open", 0, 0, "")

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: app.width / 2, Y: app.height - 2, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Nil(t, cmd)
	assert.Equal(t, actionApproval, updated.actions.mode)
	assert.False(t, updated.actions.submitting)
}

func TestMouseClickTopLeftDoesNotCloseCommandPalette(t *testing.T) {
	app := mouseTestApp(t)
	_ = app.cmdPalette.open(app.state, app.focus, app.showDetail, app.list.hasMore, true, app.perms)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Nil(t, cmd)
	assert.True(t, updated.cmdPalette.active)
}

func TestMouseClickCommandPaletteItemTriggersSelectedAction(t *testing.T) {
	app := mouseTestApp(t)
	_ = app.cmdPalette.open(app.state, app.focus, app.showDetail, app.list.hasMore, true, app.perms)
	optionRegion := requirePaletteActionRegion(t, app, "draft")

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: optionRegion.X1, Y: optionRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)
	require.NotNil(t, cmd)
	model, _ = updated.Update(cmd())
	updated = model.(App)

	assert.False(t, updated.cmdPalette.active)
	assert.True(t, updated.draftBusy)
}

func TestDisabledHitRegionSetsNoticeNotMergeError(t *testing.T) {
	app := mouseTestApp(t)
	app.operator.setAssets(triage.Manifest{TicketID: 1, Assets: []triage.AssetRecord{{Filename: "bad.bmp", Skipped: true, SkipReason: "unsupported image type"}}}, nil)
	assetRegion := requireRegion(t, app.hitRegions(), hitAssetFile)

	model, cmd := app.Update(tea.MouseClickMsg(tea.Mouse{X: assetRegion.X1, Y: assetRegion.Y1, Button: tea.MouseLeft}))
	updated := model.(App)

	assert.Nil(t, cmd)
	assert.Equal(t, "unsupported image type", updated.notice)
	assert.Nil(t, updated.mergeErr)
}

func requireActionOptionRegion(t *testing.T, regions []hitRegion, index int) hitRegion {
	t.Helper()
	for _, region := range regions {
		if region.Action == hitActionOption && region.TicketIndex == index {
			return region
		}
	}
	t.Fatalf("missing action option region %d in %#v", index, regions)
	return hitRegion{}
}

func requireQueueRegion(t *testing.T, regions []hitRegion, index int) hitRegion {
	t.Helper()
	for _, region := range regions {
		if region.Action == hitQueueRow && region.TicketIndex == index {
			return region
		}
	}
	t.Fatalf("missing queue region %d in %#v", index, regions)
	return hitRegion{}
}

func requirePaletteActionRegion(t *testing.T, app App, action string) hitRegion {
	t.Helper()
	for _, region := range app.actionHitRegions() {
		if region.Action == hitActionOption &&
			region.TicketIndex >= 0 &&
			region.TicketIndex < len(app.cmdPalette.filtered) &&
			app.cmdPalette.filtered[region.TicketIndex].action == action {
			return region
		}
	}
	t.Fatalf("missing palette action region %s", action)
	return hitRegion{}
}

func mouseTestApp(t *testing.T) App {
	t.Helper()
	return mouseTestAppWithService(t, &mouseTicketService{})
}

func mouseTestAppWithService(t *testing.T, svc *mouseTicketService) App {
	t.Helper()
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

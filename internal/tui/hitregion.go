package tui

type hitAction string

const (
	hitPaneList     hitAction = "pane:list"
	hitPaneDetail   hitAction = "pane:detail"
	hitPaneOperator hitAction = "pane:operator"
	hitQueueRow     hitAction = "queue:row"
	hitFieldEdit    hitAction = "field:edit"
	hitAssetsFolder hitAction = "assets:folder"
	hitAssetFile    hitAction = "assets:file"
	hitCommand      hitAction = "command"
	hitActionSubmit hitAction = "action:submit"
	hitActionCancel hitAction = "action:cancel"
	hitActionToggle hitAction = "action:toggle"
	hitActionUp     hitAction = "action:up"
	hitActionDown   hitAction = "action:down"
	hitActionOption hitAction = "action:option"
)

type hitRegion struct {
	Action hitAction
	X1     int
	Y1     int
	X2     int
	Y2     int

	TicketIndex int
	TicketID    int64
	FieldID     int64
	Command     string
	Path        string
	Disabled    bool
	Reason      string
}

func (r hitRegion) contains(x, y int) bool {
	return x >= r.X1 && x <= r.X2 && y >= r.Y1 && y <= r.Y2
}

func findHitRegion(regions []hitRegion, x, y int) (hitRegion, bool) {
	for i := len(regions) - 1; i >= 0; i-- {
		if regions[i].contains(x, y) {
			return regions[i], true
		}
	}
	return hitRegion{}, false
}

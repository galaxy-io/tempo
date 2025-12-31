package view

import (
	"fmt"
	"time"

	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	timelineLabelWidth = 25 // Width for lane labels on the left
	timelineMinWidth   = 40 // Minimum timeline bar area width
)

// TimelineLane represents a horizontal lane in the timeline.
type TimelineLane struct {
	Name      string
	Type      temporal.EventGroupType
	Status    string
	StartTime time.Time
	EndTime   *time.Time
	Node      *temporal.EventTreeNode
}

// TimelineView displays workflow events as a horizontal Gantt-style timeline.
type TimelineView struct {
	*tview.Box
	lanes             []TimelineLane
	startTime         time.Time
	endTime           time.Time
	scrollX           int
	scrollY           int
	zoomLevel         float64
	selectedLane      int
	onSelect          func(lane *TimelineLane)
	onSelectionChange func(lane *TimelineLane)
}

// NewTimelineView creates a new timeline/Gantt chart view.
func NewTimelineView() *TimelineView {
	tv := &TimelineView{
		Box:          tview.NewBox(),
		lanes:        []TimelineLane{},
		zoomLevel:    1.0,
		selectedLane: 0,
	}

	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetBorder(false)

	return tv
}

// Destroy is a no-op kept for backward compatibility.
func (tv *TimelineView) Destroy() {}

// SetNodes populates the timeline from event tree nodes.
func (tv *TimelineView) SetNodes(nodes []*temporal.EventTreeNode) {
	tv.lanes = nil
	tv.selectedLane = 0

	if len(nodes) == 0 {
		return
	}

	// First pass: collect valid lanes and find time range
	var validLanes []TimelineLane
	var minStart, maxEnd time.Time
	firstValid := true

	for _, node := range nodes {
		// Skip workflow-level events, only show activities/timers/child workflows
		if node.Type == temporal.GroupWorkflow || node.Type == temporal.GroupWorkflowTask {
			continue
		}

		// Skip nodes with zero/invalid start time
		if node.StartTime.IsZero() {
			continue
		}

		lane := TimelineLane{
			Name:      node.Name,
			Type:      node.Type,
			Status:    node.Status,
			StartTime: node.StartTime,
			EndTime:   node.EndTime,
			Node:      node,
		}
		validLanes = append(validLanes, lane)

		// Update time range
		if firstValid || node.StartTime.Before(minStart) {
			minStart = node.StartTime
		}
		if node.EndTime != nil && (firstValid || node.EndTime.After(maxEnd)) {
			maxEnd = *node.EndTime
		}
		firstValid = false
	}

	if len(validLanes) == 0 {
		return
	}

	tv.lanes = validLanes
	tv.startTime = minStart

	// Set end time: use max end time, or now for running items
	if maxEnd.IsZero() || maxEnd.Before(minStart) {
		tv.endTime = time.Now()
	} else {
		tv.endTime = maxEnd
	}

	// Ensure we have at least some time range
	if tv.endTime.Sub(tv.startTime) < time.Second {
		tv.endTime = tv.startTime.Add(time.Minute)
	}
}

// Draw renders the timeline view.
// Colors are read dynamically at draw time.
func (tv *TimelineView) Draw(screen tcell.Screen) {
	// Read colors dynamically
	bgColor := theme.Bg()
	tv.SetBackgroundColor(bgColor)

	tv.Box.DrawForSubclass(screen, tv)

	x, y, width, height := tv.GetInnerRect()
	if width < timelineLabelWidth+10 || height < 3 {
		return
	}

	// Draw header with time scale
	tv.drawHeader(screen, x, y, width)

	// Draw lanes starting from y+2 (after header)
	barAreaWidth := width - timelineLabelWidth - 1
	if barAreaWidth < timelineMinWidth {
		barAreaWidth = timelineMinWidth
	}

	timeRange := tv.endTime.Sub(tv.startTime)
	if timeRange <= 0 {
		timeRange = time.Minute
	}

	visibleLanes := height - 3 // Subtract header rows
	startLane := tv.scrollY
	endLane := startLane + visibleLanes
	if endLane > len(tv.lanes) {
		endLane = len(tv.lanes)
	}

	barStartX := x + timelineLabelWidth + 1

	for i := startLane; i < endLane; i++ {
		lane := tv.lanes[i]
		laneY := y + 2 + (i - startLane)

		// Draw lane label
		tv.drawLaneLabel(screen, x, laneY, lane, i == tv.selectedLane)

		// Draw lane bar
		tv.drawLaneBar(screen, barStartX, laneY, barAreaWidth, lane, timeRange, i == tv.selectedLane)
	}

	// Draw cursor line for selected lane
	if tv.selectedLane >= 0 && tv.selectedLane < len(tv.lanes) {
		tv.drawCursor(screen, barStartX, y, barAreaWidth, height, timeRange)
	}

	// Draw legend at bottom if space
	if height > len(tv.lanes)+4 {
		tv.drawLegend(screen, x, y+height-1, width)
	}
}

// drawHeader draws the time scale header.
func (tv *TimelineView) drawHeader(screen tcell.Screen, x, y, width int) {
	barAreaWidth := width - timelineLabelWidth - 1
	if barAreaWidth <= 0 {
		return
	}

	timeRange := tv.endTime.Sub(tv.startTime)
	if timeRange <= 0 {
		return
	}

	// Draw label column header
	labelStyle := tcell.StyleDefault.Foreground(theme.PanelTitle()).Background(theme.Bg())
	tview.Print(screen, "Event", x, y, timelineLabelWidth, tview.AlignLeft, theme.PanelTitle())

	// Draw time markers (scaled to width, with rounding)
	markerCount := 5
	if barAreaWidth < 60 {
		markerCount = 3
	}

	barStartX := x + timelineLabelWidth + 1
	zoomLevel := tv.zoomLevel
	if zoomLevel < 0.1 {
		zoomLevel = 0.1
	}

	for i := 0; i <= markerCount; i++ {
		// Calculate position with zoom and scroll
		rawPos := barAreaWidth * i / markerCount
		pos := barStartX + int(float64(rawPos)*zoomLevel) - tv.scrollX

		if pos < barStartX || pos >= x+width {
			continue
		}

		// Calculate time at this position (accounting for scroll/zoom)
		effectivePos := int(float64(pos-barStartX+tv.scrollX) / zoomLevel)
		if effectivePos < 0 {
			effectivePos = 0
		}
		if effectivePos > barAreaWidth {
			effectivePos = barAreaWidth
		}
		offset := time.Duration(float64(timeRange) * float64(effectivePos) / float64(barAreaWidth))

		// Round to nice value
		offset = roundDuration(offset)

		// Format as relative duration from start
		marker := formatRelativeDuration(offset)

		// Draw marker
		tview.Print(screen, marker, pos, y, 10, tview.AlignLeft, theme.FgDim())

		// Draw tick mark
		screen.SetContent(pos, y+1, '│', nil, labelStyle)
	}

	// Draw horizontal line under header
	lineStyle := tcell.StyleDefault.Foreground(theme.Border()).Background(theme.Bg())
	for i := x + timelineLabelWidth + 1; i < x+width; i++ {
		screen.SetContent(i, y+1, '─', nil, lineStyle)
	}
}

// drawLaneLabel draws the label for a lane.
func (tv *TimelineView) drawLaneLabel(screen tcell.Screen, x, y int, lane TimelineLane, selected bool) {
	// Truncate name if needed
	name := lane.Name
	maxLen := timelineLabelWidth - 2
	if len(name) > maxLen {
		name = name[:maxLen-1] + "…"
	}

	// Choose style based on selection
	var style tcell.Style
	if selected {
		style = tcell.StyleDefault.Foreground(theme.SelectionFg()).Background(theme.SelectionBg()).Bold(true)
	} else {
		style = tcell.StyleDefault.Foreground(tv.statusColor(lane.Status)).Background(theme.Bg())
	}

	// Clear label area
	for i := 0; i < timelineLabelWidth; i++ {
		screen.SetContent(x+i, y, ' ', nil, style)
	}

	// Draw name
	for i, r := range name {
		if x+i >= x+timelineLabelWidth {
			break
		}
		screen.SetContent(x+i, y, r, nil, style)
	}

	// Draw separator
	sepStyle := tcell.StyleDefault.Foreground(theme.Border()).Background(theme.Bg())
	screen.SetContent(x+timelineLabelWidth, y, '│', nil, sepStyle)
}

// drawLaneBar draws the timeline bar for a lane.
func (tv *TimelineView) drawLaneBar(screen tcell.Screen, x, y, width int, lane TimelineLane, timeRange time.Duration, selected bool) {
	if timeRange <= 0 || width <= 0 {
		return
	}

	// Calculate bar position and width
	startOffset := lane.StartTime.Sub(tv.startTime)
	barStart := int(float64(width) * float64(startOffset) / float64(timeRange))

	var barEnd int
	if lane.EndTime != nil {
		endOffset := lane.EndTime.Sub(tv.startTime)
		barEnd = int(float64(width) * float64(endOffset) / float64(timeRange))
	} else {
		// Running - extend to current time or end of view
		barEnd = width
	}

	// Ensure minimum bar width
	if barEnd <= barStart {
		barEnd = barStart + 1
	}

	// Apply zoom and scroll
	barStart = int(float64(barStart)*tv.zoomLevel) - tv.scrollX
	barEnd = int(float64(barEnd)*tv.zoomLevel) - tv.scrollX

	// Clamp to visible area
	if barStart < 0 {
		barStart = 0
	}
	if barEnd > width {
		barEnd = width
	}

	// Choose bar character and color based on status
	barChar, barColor := tv.barStyle(lane.Status)
	barStyle := tcell.StyleDefault.Foreground(barColor).Background(theme.Bg())

	if selected {
		barStyle = barStyle.Bold(true)
	}

	// Draw empty space before bar
	emptyStyle := tcell.StyleDefault.Foreground(theme.BgLight()).Background(theme.Bg())
	for i := 0; i < barStart && i < width; i++ {
		screen.SetContent(x+i, y, '·', nil, emptyStyle)
	}

	// Draw the bar
	for i := barStart; i < barEnd && i < width; i++ {
		screen.SetContent(x+i, y, barChar, nil, barStyle)
	}

	// Draw empty space after bar
	for i := barEnd; i < width; i++ {
		screen.SetContent(x+i, y, '·', nil, emptyStyle)
	}
}

// drawCursor draws a candlestick-style cursor showing gap and duration for selected lane.
func (tv *TimelineView) drawCursor(screen tcell.Screen, x, y, width, height int, timeRange time.Duration) {
	if timeRange <= 0 || width <= 0 {
		return
	}

	lane := tv.lanes[tv.selectedLane]

	// Calculate positions
	startOffset := lane.StartTime.Sub(tv.startTime)
	startPos := int(float64(width) * float64(startOffset) / float64(timeRange))

	var endPos int
	if lane.EndTime != nil {
		endOffset := lane.EndTime.Sub(tv.startTime)
		endPos = int(float64(width) * float64(endOffset) / float64(timeRange))
	} else {
		endPos = width // Running - extends to end
	}

	// Find previous lane's end time for gap calculation
	var prevEndPos int
	if tv.selectedLane > 0 {
		prevLane := tv.lanes[tv.selectedLane-1]
		if prevLane.EndTime != nil {
			prevEndOffset := prevLane.EndTime.Sub(tv.startTime)
			prevEndPos = int(float64(width) * float64(prevEndOffset) / float64(timeRange))
		}
	}

	// Apply zoom and scroll
	startPos = int(float64(startPos)*tv.zoomLevel) - tv.scrollX
	endPos = int(float64(endPos)*tv.zoomLevel) - tv.scrollX
	prevEndPos = int(float64(prevEndPos)*tv.zoomLevel) - tv.scrollX

	// Calculate the row for the selected lane
	selectedRow := y + 2 + (tv.selectedLane - tv.scrollY)
	lanesEnd := y + 2 + (len(tv.lanes) - tv.scrollY)
	if lanesEnd > y+height-1 {
		lanesEnd = y + height - 1
	}

	// Draw gap "wick" from previous end to current start (thin line)
	if prevEndPos > 0 && prevEndPos < startPos {
		wickStyle := tcell.StyleDefault.Foreground(theme.FgDim()).Background(theme.Bg())
		for col := prevEndPos; col < startPos && col < width; col++ {
			if col >= 0 {
				screen.SetContent(x+col, selectedRow, '─', nil, wickStyle)
			}
		}
	}

	// Draw vertical cursor line at start position
	if startPos >= 0 && startPos < width {
		cursorStyle := tcell.StyleDefault.Foreground(theme.Accent()).Background(theme.Bg())
		for row := y + 2; row < lanesEnd; row++ {
			screen.SetContent(x+startPos, row, '│', nil, cursorStyle)
		}

		// Draw start time label in header
		startLabel := formatRelativeDuration(startOffset)
		labelStyle := tcell.StyleDefault.Foreground(theme.Bg()).Background(theme.Accent())
		labelX := x + startPos
		if labelX+len(startLabel) > x+width {
			labelX = x + width - len(startLabel)
		}
		if labelX < x {
			labelX = x
		}
		for i, r := range startLabel {
			if labelX+i >= x && labelX+i < x+width {
				screen.SetContent(labelX+i, y, r, nil, labelStyle)
			}
		}
	}

	// Only draw end marker and duration for completed items (those with an EndTime)
	if lane.EndTime != nil {
		duration := lane.EndTime.Sub(lane.StartTime)
		durationLabel := formatRelativeDuration(duration)
		durationStyle := tcell.StyleDefault.Foreground(theme.Bg()).Background(theme.Success())

		// Draw vertical line at end position (if visible and different from start)
		if endPos > startPos && endPos >= 0 && endPos < width {
			endStyle := tcell.StyleDefault.Foreground(theme.Success()).Background(theme.Bg())
			for row := y + 2; row < lanesEnd; row++ {
				screen.SetContent(x+endPos, row, '│', nil, endStyle)
			}
		}

		// Calculate available space inside the candlestick
		candleWidth := endPos - startPos
		startLabelLen := len(formatRelativeDuration(startOffset))

		var durationX int
		if candleWidth > len(durationLabel)+2 {
			// Fits inside - center it between start and end
			midPos := (startPos + endPos) / 2
			durationX = x + midPos - len(durationLabel)/2
			// Make sure it doesn't overlap with start label
			if durationX < x+startPos+startLabelLen+1 {
				durationX = x + startPos + startLabelLen + 1
			}
		} else {
			// Doesn't fit inside - place it to the right of end marker
			durationX = x + endPos + 1
		}

		// Clamp to screen bounds
		if durationX < x {
			durationX = x
		}
		if durationX+len(durationLabel) > x+width {
			durationX = x + width - len(durationLabel)
		}

		// Draw the duration label if it fits on screen
		if durationX >= x && durationX < x+width {
			for i, r := range durationLabel {
				if durationX+i >= x && durationX+i < x+width {
					screen.SetContent(durationX+i, y+1, r, nil, durationStyle)
				}
			}
		}
	}
}

// drawLegend draws the status legend and selected lane stats at the bottom.
func (tv *TimelineView) drawLegend(screen tcell.Screen, x, y, width int) {
	legend := []struct {
		char   rune
		status string
		color  tcell.Color
	}{
		{'█', "Completed", theme.Success()},
		{'▓', "Running", theme.Warning()},
		{'░', "Failed", theme.Error()},
		{'▒', "Pending", theme.FgDim()},
	}

	pos := x
	for _, item := range legend {
		if pos+12 > x+width/2 {
			break
		}

		style := tcell.StyleDefault.Foreground(item.color).Background(theme.Bg())
		screen.SetContent(pos, y, item.char, nil, style)
		pos++

		labelStyle := tcell.StyleDefault.Foreground(theme.FgDim()).Background(theme.Bg())
		for _, r := range item.status {
			screen.SetContent(pos, y, r, nil, labelStyle)
			pos++
		}
		pos += 1 // spacing
	}

	// Draw selected lane stats on the right side
	if tv.selectedLane >= 0 && tv.selectedLane < len(tv.lanes) {
		lane := tv.lanes[tv.selectedLane]

		// Calculate stats
		startOffset := lane.StartTime.Sub(tv.startTime)

		// Calculate gap from previous lane
		var gap time.Duration
		if tv.selectedLane > 0 {
			prevLane := tv.lanes[tv.selectedLane-1]
			if prevLane.EndTime != nil {
				gap = lane.StartTime.Sub(*prevLane.EndTime)
				if gap < 0 {
					gap = 0
				}
			}
		}

		// Build stats segments with their colors
		type statSegment struct {
			text  string
			color tcell.Color
		}

		segments := []statSegment{}
		labelColor := theme.FgDim()

		// Start segment (accent color)
		segments = append(segments, statSegment{"Start:", labelColor})
		segments = append(segments, statSegment{formatRelativeDuration(startOffset), theme.Accent()})
		segments = append(segments, statSegment{"  ", labelColor})

		// Duration or running segment
		if lane.EndTime != nil {
			duration := lane.EndTime.Sub(lane.StartTime)
			segments = append(segments, statSegment{"Dur:", labelColor})
			segments = append(segments, statSegment{formatRelativeDuration(duration), theme.Success()})
		} else {
			segments = append(segments, statSegment{"(running)", theme.Warning()})
		}

		// Gap segment (dim color)
		if gap > 0 {
			segments = append(segments, statSegment{"  Gap:", labelColor})
			segments = append(segments, statSegment{formatRelativeDuration(gap), theme.FgDim()})
		}

		// Calculate total length
		totalLen := 0
		for _, seg := range segments {
			totalLen += len(seg.text)
		}

		// Draw stats right-aligned
		statsX := x + width - totalLen
		if statsX < pos+2 {
			statsX = pos + 2
		}

		currentX := statsX
		for _, seg := range segments {
			style := tcell.StyleDefault.Foreground(seg.color).Background(theme.Bg())
			for _, r := range seg.text {
				if currentX >= x+width {
					break
				}
				screen.SetContent(currentX, y, r, nil, style)
				currentX++
			}
		}
	}
}

// barStyle returns the bar character and color for a status.
func (tv *TimelineView) barStyle(status string) (rune, tcell.Color) {
	switch status {
	case "Running":
		return '▓', theme.Warning()
	case "Completed", "Fired":
		return '█', theme.Success()
	case "Failed", "TimedOut":
		return '░', theme.Error()
	case "Canceled", "Terminated":
		return '▒', theme.Warning()
	case "Scheduled", "Initiated", "Pending":
		return '▒', theme.FgDim()
	default:
		return '▒', theme.Fg()
	}
}

// statusColor returns the color for a status.
func (tv *TimelineView) statusColor(status string) tcell.Color {
	return temporal.GetWorkflowStatus(status).Color()
}

// InputHandler handles keyboard input.
func (tv *TimelineView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return tv.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyUp:
			tv.moveSelection(-1)
		case tcell.KeyDown:
			tv.moveSelection(1)
		case tcell.KeyLeft:
			tv.scroll(-5)
		case tcell.KeyRight:
			tv.scroll(5)
		case tcell.KeyEnter:
			if tv.onSelect != nil && tv.selectedLane >= 0 && tv.selectedLane < len(tv.lanes) {
				tv.onSelect(&tv.lanes[tv.selectedLane])
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				tv.moveSelection(-1)
			case 'j':
				tv.moveSelection(1)
			case 'h':
				tv.scroll(-5)
			case 'l':
				tv.scroll(5)
			case '+', '=':
				tv.zoom(1.2)
			case '-':
				tv.zoom(0.8)
			case '0':
				tv.resetView()
			}
		}
	})
}

// moveSelection moves the lane selection up or down.
func (tv *TimelineView) moveSelection(delta int) {
	if len(tv.lanes) == 0 {
		return
	}

	oldSelection := tv.selectedLane

	tv.selectedLane += delta
	if tv.selectedLane < 0 {
		tv.selectedLane = 0
	}
	if tv.selectedLane >= len(tv.lanes) {
		tv.selectedLane = len(tv.lanes) - 1
	}

	// Adjust scroll to keep selection visible
	_, _, _, height := tv.GetInnerRect()
	visibleLanes := height - 3

	if tv.selectedLane < tv.scrollY {
		tv.scrollY = tv.selectedLane
	}
	if tv.selectedLane >= tv.scrollY+visibleLanes {
		tv.scrollY = tv.selectedLane - visibleLanes + 1
	}

	// Notify if selection changed
	if tv.selectedLane != oldSelection && tv.onSelectionChange != nil {
		tv.onSelectionChange(&tv.lanes[tv.selectedLane])
	}
}

// scroll horizontally scrolls the timeline.
func (tv *TimelineView) scroll(delta int) {
	tv.scrollX += delta
	if tv.scrollX < 0 {
		tv.scrollX = 0
	}
}

// zoom adjusts the zoom level.
func (tv *TimelineView) zoom(factor float64) {
	tv.zoomLevel *= factor
	if tv.zoomLevel < 0.5 {
		tv.zoomLevel = 0.5
	}
	if tv.zoomLevel > 5.0 {
		tv.zoomLevel = 5.0
	}
}

// resetView resets zoom and scroll.
func (tv *TimelineView) resetView() {
	tv.zoomLevel = 1.0
	tv.scrollX = 0
	tv.scrollY = 0
}

// SetOnSelect sets the callback for lane selection (Enter key).
func (tv *TimelineView) SetOnSelect(fn func(lane *TimelineLane)) {
	tv.onSelect = fn
}

// SetOnSelectionChange sets the callback for when selection changes (navigation).
func (tv *TimelineView) SetOnSelectionChange(fn func(lane *TimelineLane)) {
	tv.onSelectionChange = fn
}

// SelectedLane returns the currently selected lane.
func (tv *TimelineView) SelectedLane() *TimelineLane {
	if tv.selectedLane >= 0 && tv.selectedLane < len(tv.lanes) {
		return &tv.lanes[tv.selectedLane]
	}
	return nil
}

// LaneCount returns the number of lanes.
func (tv *TimelineView) LaneCount() int {
	return len(tv.lanes)
}

// Focus implements tview.Primitive.
func (tv *TimelineView) Focus(delegate func(p tview.Primitive)) {
	tv.Box.Focus(delegate)
}

// HasFocus implements tview.Primitive.
func (tv *TimelineView) HasFocus() bool {
	return tv.Box.HasFocus()
}

// roundDuration rounds a duration up to a nice value.
func roundDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}

	// Define rounding thresholds and their round-to values
	type roundRule struct {
		threshold time.Duration
		roundTo   time.Duration
	}

	rules := []roundRule{
		{100 * time.Millisecond, 10 * time.Millisecond},   // < 100ms: round to 10ms
		{time.Second, 50 * time.Millisecond},              // < 1s: round to 50ms
		{10 * time.Second, 500 * time.Millisecond},        // < 10s: round to 500ms
		{time.Minute, time.Second},                        // < 1m: round to 1s
		{10 * time.Minute, 10 * time.Second},              // < 10m: round to 10s
		{time.Hour, time.Minute},                          // < 1h: round to 1m
		{24 * time.Hour, 10 * time.Minute},                // < 24h: round to 10m
	}

	for _, rule := range rules {
		if d < rule.threshold {
			// Round up to nearest roundTo
			return ((d + rule.roundTo - 1) / rule.roundTo) * rule.roundTo
		}
	}

	// For very long durations, round to nearest hour
	return ((d + time.Hour - 1) / time.Hour) * time.Hour
}

// formatRelativeDuration formats a duration as a relative time string.
func formatRelativeDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		secs := d.Seconds()
		if secs == float64(int(secs)) {
			return fmt.Sprintf("%ds", int(secs))
		}
		return fmt.Sprintf("%.1fs", secs)
	}
	if d < time.Hour {
		mins := d.Minutes()
		if mins == float64(int(mins)) {
			return fmt.Sprintf("%dm", int(mins))
		}
		return fmt.Sprintf("%.1fm", mins)
	}
	hours := d.Hours()
	if hours == float64(int(hours)) {
		return fmt.Sprintf("%dh", int(hours))
	}
	return fmt.Sprintf("%.1fh", hours)
}

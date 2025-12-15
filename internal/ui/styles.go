package ui

import "github.com/gdamore/tcell/v2"

// Theme colors - Charm/Catppuccin Mocha inspired
var (
	// Base colors (Catppuccin Mocha)
	ColorBg        = tcell.NewRGBColor(0x1e, 0x1e, 0x2e) // Base
	ColorBgLight   = tcell.NewRGBColor(0x31, 0x32, 0x44) // Surface0
	ColorBgDark    = tcell.NewRGBColor(0x18, 0x18, 0x25) // Mantle
	ColorFg        = tcell.NewRGBColor(0xcd, 0xd6, 0xf4) // Text
	ColorFgDim     = tcell.NewRGBColor(0x6c, 0x70, 0x86) // Overlay0
	ColorBorder    = tcell.NewRGBColor(0x45, 0x47, 0x5a) // Surface1
	ColorHighlight = tcell.NewRGBColor(0x58, 0x5b, 0x70) // Surface2

	// Accent colors (Pink/Mauve - signature Charm look)
	ColorAccent    = tcell.NewRGBColor(0xf5, 0xc2, 0xe7) // Pink
	ColorAccentDim = tcell.NewRGBColor(0xcb, 0xa6, 0xf7) // Mauve

	// Status colors (soft pastels)
	ColorRunning    = tcell.NewRGBColor(0xf9, 0xe2, 0xaf) // Yellow
	ColorCompleted  = tcell.NewRGBColor(0xa6, 0xe3, 0xa1) // Green
	ColorFailed     = tcell.NewRGBColor(0xf3, 0x8b, 0xa8) // Red
	ColorCanceled   = tcell.NewRGBColor(0xfa, 0xb3, 0x87) // Peach
	ColorTerminated = tcell.NewRGBColor(0xcb, 0xa6, 0xf7) // Mauve
	ColorTimedOut   = tcell.NewRGBColor(0xf3, 0x8b, 0xa8) // Red

	// UI element colors
	ColorHeader   = tcell.NewRGBColor(0x18, 0x18, 0x25) // Mantle
	ColorMenu     = tcell.NewRGBColor(0x1e, 0x1e, 0x2e) // Base
	ColorTableHdr = tcell.NewRGBColor(0xf5, 0xc2, 0xe7) // Pink
	ColorKey      = tcell.NewRGBColor(0xcb, 0xa6, 0xf7) // Mauve
	ColorCrumb    = tcell.NewRGBColor(0xf5, 0xc2, 0xe7) // Pink

	// Panel colors
	ColorPanelBorder = tcell.NewRGBColor(0x58, 0x5b, 0x70) // Surface2
	ColorPanelTitle  = tcell.NewRGBColor(0xf5, 0xc2, 0xe7) // Pink
)

// Nerd Font icons (requires a Nerd Font installed)
const (
	// Status icons
	IconRunning    = "\uf144" // nf-fa-play_circle
	IconCompleted  = "\uf00c" // nf-fa-check
	IconFailed     = "\uf00d" // nf-fa-times
	IconCanceled   = "\uf05e" // nf-fa-ban
	IconTerminated = "\uf28d" // nf-fa-stop_circle
	IconTimedOut   = "\uf017" // nf-fa-clock_o
	IconPending    = "\uf10c" // nf-fa-circle_o

	// Navigation
	IconArrowRight = "\uf054" // nf-fa-chevron_right
	IconArrowDown  = "\uf078" // nf-fa-chevron_down
	IconArrowUp    = "\uf077" // nf-fa-chevron_up
	IconBullet     = "\uf192" // nf-fa-dot_circle_o
	IconDot        = "\uf111" // nf-fa-circle

	// Separators
	IconSeparator = "\uf105" // nf-fa-angle_right
	IconDash      = "\uf068" // nf-fa-minus

	// Indicators
	IconConnected    = "\uf1e6" // nf-fa-plug
	IconDisconnected = "\uf127" // nf-fa-chain_broken
	IconActivity     = "\uf013" // nf-fa-cog
	IconWorkflow     = "\uf0e7" // nf-fa-bolt
	IconNamespace    = "\uf0c9" // nf-fa-bars
	IconTaskQueue    = "\uf0ae" // nf-fa-tasks
	IconEvent        = "\uf1da" // nf-fa-history

	// Box drawing
	BoxTopLeft     = "\u256d"
	BoxTopRight    = "\u256e"
	BoxBottomLeft  = "\u2570"
	BoxBottomRight = "\u256f"
	BoxHorizontal  = "\u2500"
	BoxVertical    = "\u2502"
)

// Logo for the header
const Logo = `temporal-tui`

// LogoSmall is a compact version
const LogoSmall = "temporal-tui"

// StatusIcon returns the icon for a workflow status.
func StatusIcon(status string) string {
	switch status {
	case "Running":
		return IconRunning
	case "Completed":
		return IconCompleted
	case "Failed":
		return IconFailed
	case "Canceled":
		return IconCanceled
	case "Terminated":
		return IconTerminated
	case "TimedOut":
		return IconTimedOut
	default:
		return IconPending
	}
}

// StatusColorTcell returns the tcell color for a workflow status.
func StatusColorTcell(status string) tcell.Color {
	switch status {
	case "Running":
		return ColorRunning
	case "Completed":
		return ColorCompleted
	case "Failed":
		return ColorFailed
	case "Canceled":
		return ColorCanceled
	case "Terminated":
		return ColorTerminated
	case "TimedOut":
		return ColorTimedOut
	default:
		return ColorFg
	}
}

// StatusColorTag returns the tview color tag for a status.
func StatusColorTag(status string) string {
	switch status {
	case "Running":
		return "#f9e2af" // Yellow
	case "Completed":
		return "#a6e3a1" // Green
	case "Failed":
		return "#f38ba8" // Red
	case "Canceled":
		return "#fab387" // Peach
	case "Terminated":
		return "#cba6f7" // Mauve
	case "TimedOut":
		return "#f38ba8" // Red
	default:
		return "#cdd6f4" // Text
	}
}

// Color tags for tview dynamic colors (Catppuccin Mocha)
const (
	TagBg        = "#1e1e2e"
	TagFg        = "#cdd6f4"
	TagFgDim     = "#6c7086"
	TagAccent    = "#f5c2e7" // Pink
	TagKey       = "#cba6f7" // Mauve
	TagCrumb     = "#f5c2e7" // Pink
	TagTableHdr  = "#f5c2e7" // Pink
	TagHighlight = "#585b70"
	TagBorder    = "#45475a"

	// Status tags
	TagRunning   = "#f9e2af" // Yellow
	TagCompleted = "#a6e3a1" // Green
	TagFailed    = "#f38ba8" // Red
	TagCanceled  = "#fab387" // Peach

	// Panel tags
	TagPanelBorder = "#585b70" // Surface2
	TagPanelTitle  = "#f5c2e7" // Pink
)

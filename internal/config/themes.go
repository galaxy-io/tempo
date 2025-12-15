package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ThemeColors holds all color values for a theme as hex strings.
type ThemeColors struct {
	// Base colors
	Bg        string `yaml:"bg"`
	BgLight   string `yaml:"bg_light"`
	BgDark    string `yaml:"bg_dark"`
	Fg        string `yaml:"fg"`
	FgDim     string `yaml:"fg_dim"`
	Border    string `yaml:"border"`
	Highlight string `yaml:"highlight"`

	// Accent colors
	Accent    string `yaml:"accent"`
	AccentDim string `yaml:"accent_dim"`

	// Status colors
	Running    string `yaml:"running"`
	Completed  string `yaml:"completed"`
	Failed     string `yaml:"failed"`
	Canceled   string `yaml:"canceled"`
	Terminated string `yaml:"terminated"`
	TimedOut   string `yaml:"timed_out"`

	// UI element colors
	Header      string `yaml:"header"`
	Menu        string `yaml:"menu"`
	TableHeader string `yaml:"table_header"`
	Key         string `yaml:"key"`
	Crumb       string `yaml:"crumb"`
	PanelBorder string `yaml:"panel_border"`
	PanelTitle  string `yaml:"panel_title"`
}

// Theme represents a color theme definition.
type Theme struct {
	Name   string      `yaml:"name"`
	Type   string      `yaml:"type"` // "dark" or "light"
	Colors ThemeColors `yaml:"colors"`
}

// ParsedColors holds parsed tcell.Color values ready for use.
type ParsedColors struct {
	Bg        tcell.Color
	BgLight   tcell.Color
	BgDark    tcell.Color
	Fg        tcell.Color
	FgDim     tcell.Color
	Border    tcell.Color
	Highlight tcell.Color

	Accent    tcell.Color
	AccentDim tcell.Color

	Running    tcell.Color
	Completed  tcell.Color
	Failed     tcell.Color
	Canceled   tcell.Color
	Terminated tcell.Color
	TimedOut   tcell.Color

	Header      tcell.Color
	Menu        tcell.Color
	TableHeader tcell.Color
	Key         tcell.Color
	Crumb       tcell.Color
	PanelBorder tcell.Color
	PanelTitle  tcell.Color
}

// ParsedTheme combines theme metadata with parsed colors.
type ParsedTheme struct {
	Key    string       // Theme identifier (e.g., "tokyonight-night")
	Name   string       // Display name (e.g., "TokyoNight Night")
	Type   string       // "dark" or "light"
	Colors ParsedColors
	Tags   ThemeColors // Keep original hex for tview tags
}

// parseHexColor converts a hex color string to tcell.Color.
func parseHexColor(hex string) (tcell.Color, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return tcell.ColorDefault, fmt.Errorf("invalid hex color: %s", hex)
	}

	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return tcell.ColorDefault, err
	}
	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return tcell.ColorDefault, err
	}
	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return tcell.ColorDefault, err
	}

	return tcell.NewRGBColor(int32(r), int32(g), int32(b)), nil
}

// Parse converts a Theme to a ParsedTheme with tcell.Color values.
func (t *Theme) Parse() (*ParsedTheme, error) {
	p := &ParsedTheme{
		Name: t.Name,
		Type: t.Type,
		Tags: t.Colors,
	}

	var err error

	if p.Colors.Bg, err = parseHexColor(t.Colors.Bg); err != nil {
		return nil, fmt.Errorf("bg: %w", err)
	}
	if p.Colors.BgLight, err = parseHexColor(t.Colors.BgLight); err != nil {
		return nil, fmt.Errorf("bg_light: %w", err)
	}
	if p.Colors.BgDark, err = parseHexColor(t.Colors.BgDark); err != nil {
		return nil, fmt.Errorf("bg_dark: %w", err)
	}
	if p.Colors.Fg, err = parseHexColor(t.Colors.Fg); err != nil {
		return nil, fmt.Errorf("fg: %w", err)
	}
	if p.Colors.FgDim, err = parseHexColor(t.Colors.FgDim); err != nil {
		return nil, fmt.Errorf("fg_dim: %w", err)
	}
	if p.Colors.Border, err = parseHexColor(t.Colors.Border); err != nil {
		return nil, fmt.Errorf("border: %w", err)
	}
	if p.Colors.Highlight, err = parseHexColor(t.Colors.Highlight); err != nil {
		return nil, fmt.Errorf("highlight: %w", err)
	}
	if p.Colors.Accent, err = parseHexColor(t.Colors.Accent); err != nil {
		return nil, fmt.Errorf("accent: %w", err)
	}
	if p.Colors.AccentDim, err = parseHexColor(t.Colors.AccentDim); err != nil {
		return nil, fmt.Errorf("accent_dim: %w", err)
	}
	if p.Colors.Running, err = parseHexColor(t.Colors.Running); err != nil {
		return nil, fmt.Errorf("running: %w", err)
	}
	if p.Colors.Completed, err = parseHexColor(t.Colors.Completed); err != nil {
		return nil, fmt.Errorf("completed: %w", err)
	}
	if p.Colors.Failed, err = parseHexColor(t.Colors.Failed); err != nil {
		return nil, fmt.Errorf("failed: %w", err)
	}
	if p.Colors.Canceled, err = parseHexColor(t.Colors.Canceled); err != nil {
		return nil, fmt.Errorf("canceled: %w", err)
	}
	if p.Colors.Terminated, err = parseHexColor(t.Colors.Terminated); err != nil {
		return nil, fmt.Errorf("terminated: %w", err)
	}
	if p.Colors.TimedOut, err = parseHexColor(t.Colors.TimedOut); err != nil {
		return nil, fmt.Errorf("timed_out: %w", err)
	}
	if p.Colors.Header, err = parseHexColor(t.Colors.Header); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if p.Colors.Menu, err = parseHexColor(t.Colors.Menu); err != nil {
		return nil, fmt.Errorf("menu: %w", err)
	}
	if p.Colors.TableHeader, err = parseHexColor(t.Colors.TableHeader); err != nil {
		return nil, fmt.Errorf("table_header: %w", err)
	}
	if p.Colors.Key, err = parseHexColor(t.Colors.Key); err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}
	if p.Colors.Crumb, err = parseHexColor(t.Colors.Crumb); err != nil {
		return nil, fmt.Errorf("crumb: %w", err)
	}
	if p.Colors.PanelBorder, err = parseHexColor(t.Colors.PanelBorder); err != nil {
		return nil, fmt.Errorf("panel_border: %w", err)
	}
	if p.Colors.PanelTitle, err = parseHexColor(t.Colors.PanelTitle); err != nil {
		return nil, fmt.Errorf("panel_title: %w", err)
	}

	return p, nil
}

// BuiltinThemes contains all predefined themes.
var BuiltinThemes = map[string]*Theme{
	// TokyoNight variants
	"tokyonight-night": {
		Name: "TokyoNight Night",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#1a1b26",
			BgLight:     "#24283b",
			BgDark:      "#16161e",
			Fg:          "#c0caf5",
			FgDim:       "#565f89",
			Border:      "#15161e",
			Highlight:   "#283457",
			Accent:      "#7aa2f7",
			AccentDim:   "#bb9af7",
			Running:     "#e0af68",
			Completed:   "#9ece6a",
			Failed:      "#f7768e",
			Canceled:    "#ff9e64",
			Terminated:  "#bb9af7",
			TimedOut:    "#f7768e",
			Header:      "#16161e",
			Menu:        "#1a1b26",
			TableHeader: "#7aa2f7",
			Key:         "#bb9af7",
			Crumb:       "#7aa2f7",
			PanelBorder: "#283457",
			PanelTitle:  "#7aa2f7",
		},
	},
	"tokyonight-storm": {
		Name: "TokyoNight Storm",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#24283b",
			BgLight:     "#292e42",
			BgDark:      "#1f2335",
			Fg:          "#c0caf5",
			FgDim:       "#565f89",
			Border:      "#1d202f",
			Highlight:   "#292e42",
			Accent:      "#7aa2f7",
			AccentDim:   "#bb9af7",
			Running:     "#e0af68",
			Completed:   "#9ece6a",
			Failed:      "#f7768e",
			Canceled:    "#ff9e64",
			Terminated:  "#bb9af7",
			TimedOut:    "#f7768e",
			Header:      "#1f2335",
			Menu:        "#24283b",
			TableHeader: "#7aa2f7",
			Key:         "#bb9af7",
			Crumb:       "#7aa2f7",
			PanelBorder: "#292e42",
			PanelTitle:  "#7aa2f7",
		},
	},
	"tokyonight-moon": {
		Name: "TokyoNight Moon",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#222436",
			BgLight:     "#2f334d",
			BgDark:      "#1e2030",
			Fg:          "#c8d3f5",
			FgDim:       "#636da6",
			Border:      "#1b1d2b",
			Highlight:   "#2f334d",
			Accent:      "#82aaff",
			AccentDim:   "#c099ff",
			Running:     "#ffc777",
			Completed:   "#c3e88d",
			Failed:      "#ff757f",
			Canceled:    "#ff966c",
			Terminated:  "#c099ff",
			TimedOut:    "#ff757f",
			Header:      "#1e2030",
			Menu:        "#222436",
			TableHeader: "#82aaff",
			Key:         "#c099ff",
			Crumb:       "#82aaff",
			PanelBorder: "#2f334d",
			PanelTitle:  "#82aaff",
		},
	},
	"tokyonight-day": {
		Name: "TokyoNight Day",
		Type: "light",
		Colors: ThemeColors{
			Bg:          "#e1e2e7",
			BgLight:     "#d0d5e3",
			BgDark:      "#b4b5b9",
			Fg:          "#3760bf",
			FgDim:       "#848cb5",
			Border:      "#b4b5b9",
			Highlight:   "#b7c1e3",
			Accent:      "#2e7de9",
			AccentDim:   "#9854f1",
			Running:     "#8c6c3e",
			Completed:   "#587539",
			Failed:      "#f52a65",
			Canceled:    "#b15c00",
			Terminated:  "#7847bd",
			TimedOut:    "#f52a65",
			Header:      "#d0d5e3",
			Menu:        "#e1e2e7",
			TableHeader: "#2e7de9",
			Key:         "#9854f1",
			Crumb:       "#2e7de9",
			PanelBorder: "#b4b5b9",
			PanelTitle:  "#2e7de9",
		},
	},

	// Catppuccin variants
	"catppuccin-mocha": {
		Name: "Catppuccin Mocha",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#1e1e2e",
			BgLight:     "#313244",
			BgDark:      "#181825",
			Fg:          "#cdd6f4",
			FgDim:       "#6c7086",
			Border:      "#45475a",
			Highlight:   "#585b70",
			Accent:      "#f5c2e7",
			AccentDim:   "#cba6f7",
			Running:     "#f9e2af",
			Completed:   "#a6e3a1",
			Failed:      "#f38ba8",
			Canceled:    "#fab387",
			Terminated:  "#cba6f7",
			TimedOut:    "#f38ba8",
			Header:      "#181825",
			Menu:        "#1e1e2e",
			TableHeader: "#f5c2e7",
			Key:         "#cba6f7",
			Crumb:       "#f5c2e7",
			PanelBorder: "#585b70",
			PanelTitle:  "#f5c2e7",
		},
	},
	"catppuccin-macchiato": {
		Name: "Catppuccin Macchiato",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#24273a",
			BgLight:     "#363a4f",
			BgDark:      "#1e2030",
			Fg:          "#cad3f5",
			FgDim:       "#6e738d",
			Border:      "#494d64",
			Highlight:   "#5b6078",
			Accent:      "#f5bde6",
			AccentDim:   "#c6a0f6",
			Running:     "#eed49f",
			Completed:   "#a6da95",
			Failed:      "#ed8796",
			Canceled:    "#f5a97f",
			Terminated:  "#c6a0f6",
			TimedOut:    "#ed8796",
			Header:      "#1e2030",
			Menu:        "#24273a",
			TableHeader: "#f5bde6",
			Key:         "#c6a0f6",
			Crumb:       "#f5bde6",
			PanelBorder: "#5b6078",
			PanelTitle:  "#f5bde6",
		},
	},
	"catppuccin-frappe": {
		Name: "Catppuccin Frappé",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#303446",
			BgLight:     "#414559",
			BgDark:      "#292c3c",
			Fg:          "#c6d0f5",
			FgDim:       "#737994",
			Border:      "#51576d",
			Highlight:   "#626880",
			Accent:      "#f4b8e4",
			AccentDim:   "#ca9ee6",
			Running:     "#e5c890",
			Completed:   "#a6d189",
			Failed:      "#e78284",
			Canceled:    "#ef9f76",
			Terminated:  "#ca9ee6",
			TimedOut:    "#e78284",
			Header:      "#292c3c",
			Menu:        "#303446",
			TableHeader: "#f4b8e4",
			Key:         "#ca9ee6",
			Crumb:       "#f4b8e4",
			PanelBorder: "#626880",
			PanelTitle:  "#f4b8e4",
		},
	},
	"catppuccin-latte": {
		Name: "Catppuccin Latte",
		Type: "light",
		Colors: ThemeColors{
			Bg:          "#eff1f5",
			BgLight:     "#ccd0da",
			BgDark:      "#e6e9ef",
			Fg:          "#4c4f69",
			FgDim:       "#6c6f85",
			Border:      "#bcc0cc",
			Highlight:   "#acb0be",
			Accent:      "#ea76cb",
			AccentDim:   "#8839ef",
			Running:     "#df8e1d",
			Completed:   "#40a02b",
			Failed:      "#d20f39",
			Canceled:    "#fe640b",
			Terminated:  "#8839ef",
			TimedOut:    "#d20f39",
			Header:      "#e6e9ef",
			Menu:        "#eff1f5",
			TableHeader: "#ea76cb",
			Key:         "#8839ef",
			Crumb:       "#ea76cb",
			PanelBorder: "#acb0be",
			PanelTitle:  "#ea76cb",
		},
	},

	// Dracula variants
	"dracula": {
		Name: "Dracula",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#282a36",
			BgLight:     "#44475a",
			BgDark:      "#21222c",
			Fg:          "#f8f8f2",
			FgDim:       "#6272a4",
			Border:      "#44475a",
			Highlight:   "#44475a",
			Accent:      "#ff79c6",
			AccentDim:   "#bd93f9",
			Running:     "#f1fa8c",
			Completed:   "#50fa7b",
			Failed:      "#ff5555",
			Canceled:    "#ffb86c",
			Terminated:  "#bd93f9",
			TimedOut:    "#ff5555",
			Header:      "#21222c",
			Menu:        "#282a36",
			TableHeader: "#ff79c6",
			Key:         "#bd93f9",
			Crumb:       "#ff79c6",
			PanelBorder: "#6272a4",
			PanelTitle:  "#ff79c6",
		},
	},
	"dracula-light": {
		Name: "Dracula Light",
		Type: "light",
		Colors: ThemeColors{
			Bg:          "#fffbeb",
			BgLight:     "#cfcfde",
			BgDark:      "#f5f5f0",
			Fg:          "#1f1f1f",
			FgDim:       "#6c664b",
			Border:      "#cfcfde",
			Highlight:   "#cfcfde",
			Accent:      "#a3144d",
			AccentDim:   "#644ac9",
			Running:     "#846e15",
			Completed:   "#14710a",
			Failed:      "#cb3a2a",
			Canceled:    "#a34d14",
			Terminated:  "#644ac9",
			TimedOut:    "#cb3a2a",
			Header:      "#f5f5f0",
			Menu:        "#fffbeb",
			TableHeader: "#a3144d",
			Key:         "#644ac9",
			Crumb:       "#a3144d",
			PanelBorder: "#6c664b",
			PanelTitle:  "#a3144d",
		},
	},

	// Nord
	"nord": {
		Name: "Nord",
		Type: "dark",
		Colors: ThemeColors{
			Bg:          "#2e3440",
			BgLight:     "#3b4252",
			BgDark:      "#242933",
			Fg:          "#d8dee9",
			FgDim:       "#4c566a",
			Border:      "#4c566a",
			Highlight:   "#434c5e",
			Accent:      "#88c0d0",
			AccentDim:   "#81a1c1",
			Running:     "#ebcb8b",
			Completed:   "#a3be8c",
			Failed:      "#bf616a",
			Canceled:    "#d08770",
			Terminated:  "#b48ead",
			TimedOut:    "#bf616a",
			Header:      "#242933",
			Menu:        "#2e3440",
			TableHeader: "#88c0d0",
			Key:         "#81a1c1",
			Crumb:       "#88c0d0",
			PanelBorder: "#434c5e",
			PanelTitle:  "#88c0d0",
		},
	},
}

// DefaultTheme is the theme used when no config exists.
const DefaultTheme = "catppuccin-mocha"

// ThemeNames returns a sorted list of available built-in theme names.
func ThemeNames() []string {
	return []string{
		"catppuccin-frappe",
		"catppuccin-latte",
		"catppuccin-macchiato",
		"catppuccin-mocha",
		"dracula",
		"dracula-light",
		"nord",
		"tokyonight-day",
		"tokyonight-moon",
		"tokyonight-night",
		"tokyonight-storm",
	}
}

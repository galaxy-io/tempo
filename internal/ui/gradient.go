package ui

import (
	"fmt"
	"strings"
)

// GradientTags holds hex color strings for a gradient effect.
type GradientTags struct {
	Start  string // Hex color like "#f5c2e7"
	Middle string
	End    string
}

// DefaultGradientTags returns gradient colors from the current theme.
func DefaultGradientTags() GradientTags {
	return GradientTags{
		Start:  TagAccent(),
		Middle: TagCompleted(),
		End:    TagFgDim(),
	}
}

// AccentGradientTags returns a gradient using accent-based colors.
func AccentGradientTags() GradientTags {
	return GradientTags{
		Start:  TagAccent(),
		Middle: TagRunning(),
		End:    TagCompleted(),
	}
}

// parseHexColor parses a hex color string like "#f5c2e7" into RGB components.
func parseHexColor(hex string) (r, g, b int) {
	if len(hex) == 7 && hex[0] == '#' {
		fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	}
	return
}

// rgbToHex converts RGB values to a hex string.
func rgbToHex(r, g, b int) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// interpolateHex blends two hex colors based on a ratio (0.0 to 1.0).
func interpolateHex(hex1, hex2 string, ratio float64) string {
	r1, g1, b1 := parseHexColor(hex1)
	r2, g2, b2 := parseHexColor(hex2)

	r := int(float64(r1) + ratio*(float64(r2)-float64(r1)))
	g := int(float64(g1) + ratio*(float64(g2)-float64(g1)))
	b := int(float64(b1) + ratio*(float64(b2)-float64(b1)))

	return rgbToHex(r, g, b)
}

// interpolateGradientHex returns a color at position t (0.0 to 1.0) across a 3-color gradient.
func interpolateGradientHex(colors GradientTags, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	if t < 0.5 {
		// First half: Start -> Middle
		return interpolateHex(colors.Start, colors.Middle, t*2)
	}
	// Second half: Middle -> End
	return interpolateHex(colors.Middle, colors.End, (t-0.5)*2)
}

// ApplyHorizontalGradient applies a left-to-right gradient to ASCII art.
// Colors segments of each line based on horizontal position.
func ApplyHorizontalGradient(text string, colors GradientTags) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}

	// Find max line width
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	if maxWidth == 0 {
		return text
	}

	// Number of color segments per line
	segments := 10
	segmentWidth := maxWidth / segments
	if segmentWidth < 1 {
		segmentWidth = 1
	}

	var result strings.Builder
	for i, line := range lines {
		runes := []rune(line)
		currentSegment := -1
		for j, r := range runes {
			segment := j / segmentWidth
			if segment >= segments {
				segment = segments - 1
			}

			// Only add color tag when segment changes
			if segment != currentSegment {
				if currentSegment >= 0 {
					result.WriteString("[-]")
				}
				t := float64(segment) / float64(segments-1)
				color := interpolateGradientHex(colors, t)
				result.WriteString(fmt.Sprintf("[%s]", color))
				currentSegment = segment
			}
			result.WriteRune(r)
		}
		if currentSegment >= 0 {
			result.WriteString("[-]")
		}
		if i < len(lines)-1 {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

// ApplyVerticalGradient applies a top-to-bottom gradient to ASCII art.
// Each line gets a single color based on its vertical position.
func ApplyVerticalGradient(text string, colors GradientTags) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}

	var result strings.Builder
	for i, line := range lines {
		t := float64(i) / float64(len(lines)-1)
		if len(lines) == 1 {
			t = 0.5
		}
		color := interpolateGradientHex(colors, t)
		result.WriteString(fmt.Sprintf("[%s]%s[-]", color, line))
		if i < len(lines)-1 {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

// ApplyDiagonalGradient applies a diagonal gradient (top-left to bottom-right) to ASCII art.
// Colors segments based on combined row and column position.
func ApplyDiagonalGradient(text string, colors GradientTags) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}

	// Find max line width
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	if maxWidth == 0 {
		return text
	}

	// Number of diagonal bands
	bands := 12
	maxDiag := maxWidth + len(lines)
	bandSize := maxDiag / bands
	if bandSize < 1 {
		bandSize = 1
	}

	var result strings.Builder
	for i, line := range lines {
		runes := []rune(line)
		currentBand := -1
		for j, r := range runes {
			diagPos := i + j
			band := diagPos / bandSize
			if band >= bands {
				band = bands - 1
			}

			// Only add color tag when band changes
			if band != currentBand {
				if currentBand >= 0 {
					result.WriteString("[-]")
				}
				t := float64(band) / float64(bands-1)
				color := interpolateGradientHex(colors, t)
				result.WriteString(fmt.Sprintf("[%s]", color))
				currentBand = band
			}
			result.WriteRune(r)
		}
		if currentBand >= 0 {
			result.WriteString("[-]")
		}
		if i < len(lines)-1 {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

// ApplyReverseDiagonalGradient applies a diagonal gradient (top-right to bottom-left) to ASCII art.
func ApplyReverseDiagonalGradient(text string, colors GradientTags) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}

	// Find max line width
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	if maxWidth == 0 {
		return text
	}

	// Number of diagonal bands
	bands := 12
	maxDiag := maxWidth + len(lines)
	bandSize := maxDiag / bands
	if bandSize < 1 {
		bandSize = 1
	}

	var result strings.Builder
	for i, line := range lines {
		runes := []rune(line)
		currentBand := -1
		for j, r := range runes {
			diagPos := i + (maxWidth - j)
			band := diagPos / bandSize
			if band >= bands {
				band = bands - 1
			}

			// Only add color tag when band changes
			if band != currentBand {
				if currentBand >= 0 {
					result.WriteString("[-]")
				}
				t := float64(band) / float64(bands-1)
				color := interpolateGradientHex(colors, t)
				result.WriteString(fmt.Sprintf("[%s]", color))
				currentBand = band
			}
			result.WriteRune(r)
		}
		if currentBand >= 0 {
			result.WriteString("[-]")
		}
		if i < len(lines)-1 {
			result.WriteRune('\n')
		}
	}

	return result.String()
}

package dashboard

import (
	"html/template"
	"strings"
)

// HighlightYAML converts raw YAML text into HTML with CSS class spans for
// syntax highlighting. No JavaScript library is required.
//
// Rules applied line by line:
//  1. Lines starting with # -> yaml-comment span
//  2. Lines with "key: value" -> yaml-key + yaml-value spans
//  3. Lines starting with "- " -> yaml-list-marker + rest
//  4. Inline comments (# after value) -> split and class the comment portion
//  5. All text is HTML-escaped before wrapping in spans (XSS prevention)
func HighlightYAML(yamlText string) template.HTML {
	lines := strings.Split(yamlText, "\n")
	var sb strings.Builder

	for i, line := range lines {
		sb.WriteString(highlightLine(line))
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return template.HTML(sb.String()) // #nosec G203 -- content is HTML-escaped per-span
}

// highlightLine processes a single YAML line into HTML-escaped spans.
func highlightLine(line string) string {
	trimmed := strings.TrimRight(line, " \t")
	indent := leadingWhitespace(line)

	// Pure comment line.
	content := strings.TrimLeft(trimmed, " \t")
	if strings.HasPrefix(content, "#") {
		return indent + span("yaml-comment", trimmed[len(indent):])
	}

	// List marker line ("- ..." or "- ").
	if strings.HasPrefix(content, "- ") || content == "-" {
		marker := "- "
		rest := content[2:]
		if content == "-" {
			marker = "-"
			rest = ""
		}
		return indent + span("yaml-list-marker", marker) + highlightValue(rest)
	}

	// Key-value line ("key: value" or "key:").
	colonIdx := strings.Index(content, ":")
	if colonIdx > 0 {
		key := content[:colonIdx]
		afterColon := content[colonIdx+1:]
		keySpan := span("yaml-key", key+":")

		if afterColon == "" || afterColon == " " {
			return indent + keySpan
		}
		// afterColon starts with " " then value.
		space := ""
		value := afterColon
		if strings.HasPrefix(afterColon, " ") {
			space = " "
			value = afterColon[1:]
		}
		return indent + keySpan + space + highlightValue(value)
	}

	// Fall through: plain line (e.g., continuation of block scalar).
	return template.HTMLEscapeString(trimmed)
}

// highlightValue processes the value portion of a YAML line, handling inline comments.
func highlightValue(value string) string {
	if value == "" {
		return ""
	}
	// Split inline comment: look for " #" outside of quoted strings.
	commentIdx := findInlineComment(value)
	if commentIdx >= 0 {
		val := value[:commentIdx]
		comment := value[commentIdx:]
		return span("yaml-value", val) + span("yaml-comment", comment)
	}
	return span("yaml-value", value)
}

// findInlineComment returns the index of a " #" inline comment marker in value,
// or -1 if none found. Skips occurrences inside single or double quoted strings.
func findInlineComment(value string) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble && i > 0 && value[i-1] == ' ':
			return i - 1 // include the space before #
		}
	}
	return -1
}

// span wraps text in a <span> with the given CSS class, HTML-escaping the content.
func span(class, text string) string {
	return `<span class="` + class + `">` + template.HTMLEscapeString(text) + `</span>`
}

// leadingWhitespace returns the leading whitespace characters of a line.
func leadingWhitespace(line string) string {
	for i, c := range line {
		if c != ' ' && c != '\t' {
			return line[:i]
		}
	}
	return line
}

package web

// highlight.go reimplements YAML syntax highlighting for the admin module.
// The admin module cannot import monitoring packages (ADR-005), so this is a
// local copy of monitoring/internal/dashboard/highlight.go.
//
// PROTOTYPE-DEBT: [td-admin-110] Changes to highlighting logic must be applied
// in both this file and monitoring/internal/dashboard/highlight.go.

import (
	"html/template"
	"strings"
)

// highlightYAML converts raw YAML text into HTML with CSS class spans for
// syntax highlighting. No JavaScript library is required.
//
// Rules applied line by line:
//  1. Lines starting with # -> yaml-comment span
//  2. Lines with "key: value" -> yaml-key + yaml-value spans
//  3. Lines starting with "- " -> yaml-list-marker + rest
//  4. Inline comments (# after value) -> split and class the comment portion
//  5. All text is HTML-escaped before wrapping in spans (XSS prevention)
func highlightYAML(yamlText string) template.HTML {
	lines := strings.Split(yamlText, "\n")
	var sb strings.Builder

	for i, line := range lines {
		sb.WriteString(highlightYAMLLine(line))
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return template.HTML(sb.String()) // #nosec G203 -- content is HTML-escaped per-span
}

// highlightYAMLLine processes a single YAML line into HTML-escaped spans.
func highlightYAMLLine(line string) string {
	trimmed := strings.TrimRight(line, " \t")
	indent := yamlLeadingWhitespace(line)

	content := strings.TrimLeft(trimmed, " \t")
	if strings.HasPrefix(content, "#") {
		return indent + yamlSpan("yaml-comment", trimmed[len(indent):])
	}

	if strings.HasPrefix(content, "- ") || content == "-" {
		marker := "- "
		rest := content[2:]
		if content == "-" {
			marker = "-"
			rest = ""
		}
		return indent + yamlSpan("yaml-list-marker", marker) + yamlHighlightValue(rest)
	}

	colonIdx := strings.Index(content, ":")
	if colonIdx > 0 {
		key := content[:colonIdx]
		afterColon := content[colonIdx+1:]
		keySpan := yamlSpan("yaml-key", key+":")

		if afterColon == "" || afterColon == " " {
			return indent + keySpan
		}
		space := ""
		value := afterColon
		if strings.HasPrefix(afterColon, " ") {
			space = " "
			value = afterColon[1:]
		}
		return indent + keySpan + space + yamlHighlightValue(value)
	}

	return template.HTMLEscapeString(trimmed)
}

// yamlHighlightValue processes the value portion of a YAML line.
func yamlHighlightValue(value string) string {
	if value == "" {
		return ""
	}
	commentIdx := yamlFindInlineComment(value)
	if commentIdx >= 0 {
		val := value[:commentIdx]
		comment := value[commentIdx:]
		return yamlSpan("yaml-value", val) + yamlSpan("yaml-comment", comment)
	}
	return yamlSpan("yaml-value", value)
}

// yamlFindInlineComment returns the index of a " #" inline comment in value,
// or -1 if none found. Skips occurrences inside quoted strings.
func yamlFindInlineComment(value string) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble && i > 0 && value[i-1] == ' ':
			return i - 1
		}
	}
	return -1
}

// yamlSpan wraps text in a <span> with the given CSS class, HTML-escaping the content.
func yamlSpan(class, text string) string {
	return `<span class="` + class + `">` + template.HTMLEscapeString(text) + `</span>`
}

// yamlLeadingWhitespace returns the leading whitespace characters of a line.
func yamlLeadingWhitespace(line string) string {
	for i, c := range line {
		if c != ' ' && c != '\t' {
			return line[:i]
		}
	}
	return line
}

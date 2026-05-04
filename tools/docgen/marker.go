package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	markerBeginPrefix = "<!-- BEGIN:generated:"
	markerEndPrefix   = "<!-- END:generated:"
	markerSuffix      = " -->"
)

type marker struct {
	name string
}

func findMarkers(content string) []marker {
	var markers []marker
	seen := make(map[string]bool)
	inCodeBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if !strings.HasPrefix(trimmed, markerBeginPrefix) {
			continue
		}
		name := strings.TrimPrefix(trimmed, markerBeginPrefix)
		name = strings.TrimSuffix(name, markerSuffix)
		if name == "" {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		markers = append(markers, marker{name: name})
	}
	return markers
}

func replaceMarkerContent(content, name, generated string) string {
	beginTag := fmt.Sprintf("%s%s%s", markerBeginPrefix, name, markerSuffix)
	endTag := fmt.Sprintf("%s%s%s", markerEndPrefix, name, markerSuffix)

	beginIdx := strings.Index(content, beginTag)
	if beginIdx == -1 {
		return content
	}
	endIdx := strings.Index(content[beginIdx:], endTag)
	if endIdx == -1 {
		fmt.Fprintf(os.Stderr, "warning: unmatched BEGIN marker for %s\n", name)
		return content
	}
	endIdx += beginIdx

	var b strings.Builder
	b.WriteString(content[:beginIdx+len(beginTag)])
	b.WriteString("\n")
	if generated != "" {
		b.WriteString("\n")
		b.WriteString(generated)
		b.WriteString("\n\n")
	}
	b.WriteString(content[endIdx:])
	return b.String()
}

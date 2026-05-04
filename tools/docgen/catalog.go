package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generateToolCatalog(root string) (string, error) {
	path := filepath.Join(root, "internal", "claude", "tools.go")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read tools.go: %w", err)
	}

	groups := parseToolGroups(string(data))
	if len(groups) == 0 {
		return "_No tools found._", nil
	}

	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "**%s** (%d)\n\n", g.heading, len(g.tools))
		for _, t := range g.tools {
			fmt.Fprintf(&b, "- `%s`\n", t)
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

type toolGroup struct {
	heading string
	tools   []string
}

func parseToolGroups(source string) []toolGroup {
	var groups []toolGroup
	var current *toolGroup

	for _, line := range constBlockLines(source) {
		trimmed := strings.TrimSpace(line)
		if heading, ok := strings.CutPrefix(trimmed, "// "); ok {
			current = startGroup(&groups, current, heading)
			continue
		}
		if name, ok := extractToolName(trimmed); ok {
			if current == nil {
				current = &toolGroup{heading: "Other"}
			}
			current.tools = append(current.tools, name)
		}
	}
	if current != nil && len(current.tools) > 0 {
		groups = append(groups, *current)
	}
	return groups
}

func constBlockLines(source string) []string {
	var lines []string
	inConst := false
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "const (") {
			inConst = true
			continue
		}
		if inConst && trimmed == ")" {
			break
		}
		if inConst {
			lines = append(lines, line)
		}
	}
	return lines
}

func startGroup(groups *[]toolGroup, current *toolGroup, heading string) *toolGroup {
	if current != nil {
		*groups = append(*groups, *current)
	}
	return &toolGroup{heading: heading}
}

func extractToolName(line string) (string, bool) {
	if !strings.Contains(line, "=") || !strings.Contains(line, "\"") {
		return "", false
	}
	start := strings.Index(line, "\"")
	end := strings.LastIndex(line, "\"")
	if start == -1 || end <= start {
		return "", false
	}
	return line[start+1 : end], true
}

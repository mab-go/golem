package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generateExportedAPI(relDir, root string) (string, error) {
	if relDir == "." {
		return "", fmt.Errorf("exported-api not applicable to project root")
	}
	doc, err := goDoc(relDir, root)
	if err != nil {
		return "", err
	}
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return "_No exported symbols._", nil
	}
	var b strings.Builder
	b.WriteString("```\n")
	b.WriteString(doc)
	b.WriteString("\n```")
	return b.String(), nil
}

func generateDependencies(relDir, modulePath string, graph *depGraph) (string, error) {
	deps := graph.deps[relDir]
	if len(deps) == 0 {
		return "_No internal dependencies._", nil
	}
	var b strings.Builder
	for _, dep := range deps {
		b.WriteString(fmt.Sprintf("- [`%s`](%s/)\n", pkgName(dep), relPath(relDir, dep)))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func generateUsedBy(relDir, modulePath string, graph *depGraph) (string, error) {
	users := graph.usedBy[relDir]
	if len(users) == 0 {
		return "_Not imported by other internal packages._", nil
	}
	var b strings.Builder
	for _, user := range users {
		b.WriteString(fmt.Sprintf("- [`%s`](%s/)\n", pkgName(user), relPath(relDir, user)))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func generatePackageMap(pkgs []packageInfo, modulePath, root string) (string, error) {
	var b strings.Builder
	b.WriteString("| Package | Description | Files |\n")
	b.WriteString("|---------|-------------|-------|\n")
	for _, p := range pkgs {
		rel := strings.TrimPrefix(p.ImportPath, modulePath+"/")
		if strings.Contains(rel, "/pb") {
			continue
		}
		doc := p.Doc
		if doc == "" {
			doc = "--"
		} else {
			doc = firstSentence(doc)
		}
		pkgCol := "`" + rel + "`"
		if _, err := os.Stat(filepath.Join(root, rel, "README.md")); err == nil {
			pkgCol = "[`" + rel + "`](" + rel + "/)"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %d |\n", pkgCol, doc, len(p.GoFiles)))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func pkgName(importPath string) string {
	parts := strings.Split(importPath, "/")
	return parts[len(parts)-1]
}

func relPath(from, to string) string {
	fromParts := strings.Split(from, "/")
	toParts := strings.Split(to, "/")

	common := 0
	for i := 0; i < len(fromParts) && i < len(toParts); i++ {
		if fromParts[i] == toParts[i] {
			common++
		} else {
			break
		}
	}

	ups := len(fromParts) - common
	var parts []string
	for i := 0; i < ups; i++ {
		parts = append(parts, "..")
	}
	parts = append(parts, toParts[common:]...)
	return strings.Join(parts, "/")
}

func firstSentence(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ". "); idx != -1 {
		s = s[:idx+1]
	} else if !strings.HasSuffix(s, ".") {
		s += "."
	}
	if len(s) > 100 {
		s = s[:97] + "..."
	}
	return s
}

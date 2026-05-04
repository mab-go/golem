package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	root    string
	check   bool
	verbose bool
	pkg     string
}

func main() {
	cfg := parseFlags()
	modulePath, pkgs := loadProject(cfg.root)
	graph := buildGraph(pkgs, modulePath)
	readmes := findREADMEs(cfg.root, cfg.pkg)

	if cfg.verbose {
		fmt.Fprintf(os.Stderr, "found %d README(s)\n", len(readmes))
	}

	stale := updateREADMEs(readmes, cfg, modulePath, graph, pkgs)
	if cfg.check && stale {
		fmt.Fprintln(os.Stderr, "generated sections are stale; run 'make docs' to update")
		os.Exit(1)
	}
}

func parseFlags() config {
	root := flag.String("root", "", "project root directory (default: cwd)")
	check := flag.Bool("check", false, "dry-run: exit 1 if any section is stale")
	verbose := flag.Bool("verbose", false, "print each file/section being updated")
	pkg := flag.String("package", "", "update only this package (e.g. internal/agent)")
	flag.Parse()

	if *root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fatal("getwd: %v", err)
		}
		*root = wd
	}
	return config{root: *root, check: *check, verbose: *verbose, pkg: *pkg}
}

func loadProject(root string) (string, []packageInfo) {
	modulePath, err := readModulePath(root)
	if err != nil {
		fatal("read module path: %v", err)
	}
	pkgs, err := listPackages(root)
	if err != nil {
		fatal("list packages: %v", err)
	}
	return modulePath, pkgs
}

func updateREADMEs(readmes []string, cfg config, modulePath string, graph *depGraph, pkgs []packageInfo) bool {
	stale := false
	for _, readme := range readmes {
		relDir, _ := filepath.Rel(cfg.root, filepath.Dir(readme))
		if relDir == "" || relDir == "." {
			relDir = "."
		}
		updated, err := processREADME(readme, relDir, cfg.root, modulePath, graph, pkgs, cfg.check, cfg.verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", readme, err)
			stale = true
			continue
		}
		if updated {
			stale = true
		}
	}
	return stale
}

func readModulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("no module directive found in go.mod")
}

func findREADMEs(root, onlyPkg string) []string {
	var readmes []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if name == "node_modules" || name == ".git" || name == "dist" || name == "tools" {
				return filepath.SkipDir
			}
			return nil
		}
		if name != "README.md" {
			return nil
		}
		if onlyPkg != "" {
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			if rel != onlyPkg {
				return nil
			}
		}
		readmes = append(readmes, path)
		return nil
	})
	return readmes
}

func processREADME(path, relDir, root, modulePath string, graph *depGraph, pkgs []packageInfo, checkOnly, verbose bool) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	original := string(content)
	result := original

	for _, m := range findMarkers(result) {
		generated, err := generateSection(m.name, relDir, root, modulePath, graph, pkgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s:%s: %v\n", path, m.name, err)
			continue
		}
		result = replaceMarkerContent(result, m.name, generated)
		if verbose {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", relDir, m.name)
		}
	}

	if result == original {
		return false, nil
	}
	if checkOnly {
		fmt.Fprintf(os.Stderr, "stale: %s\n", path)
		return true, nil
	}
	if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
		return false, err
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "  updated %s\n", path)
	}
	return true, nil
}

func generateSection(name, relDir, root, modulePath string, graph *depGraph, pkgs []packageInfo) (string, error) {
	switch name {
	case "exported-api":
		return generateExportedAPI(relDir, root)
	case "dependencies":
		return generateDependencies(relDir, modulePath, graph)
	case "used-by":
		return generateUsedBy(relDir, modulePath, graph)
	case "package-map":
		return generatePackageMap(pkgs, modulePath, root)
	case "tool-catalog":
		return generateToolCatalog(root)
	default:
		return "", fmt.Errorf("unknown generated section: %s", name)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "docgen: "+format+"\n", args...)
	os.Exit(1)
}

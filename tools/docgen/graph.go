package main

import (
	"sort"
	"strings"
)

type depGraph struct {
	deps   map[string][]string
	usedBy map[string][]string
}

func buildGraph(pkgs []packageInfo, modulePath string) *depGraph {
	g := &depGraph{
		deps:   make(map[string][]string),
		usedBy: make(map[string][]string),
	}

	for _, p := range pkgs {
		var internal []string
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, modulePath+"/") {
				rel := strings.TrimPrefix(imp, modulePath+"/")
				internal = append(internal, rel)
			}
		}
		sort.Strings(internal)

		rel := strings.TrimPrefix(p.ImportPath, modulePath+"/")
		g.deps[rel] = internal

		for _, dep := range internal {
			g.usedBy[dep] = append(g.usedBy[dep], rel)
		}
	}

	for k := range g.usedBy {
		sort.Strings(g.usedBy[k])
	}
	return g
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type packageInfo struct {
	Dir        string
	ImportPath string
	Name       string
	Doc        string
	GoFiles    []string
	Imports    []string
}

func listPackages(root string) ([]packageInfo, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	var pkgs []packageInfo
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var p packageInfo
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

func goDoc(relPkg, root string) (string, error) {
	arg := "./" + relPkg + "/"
	cmd := exec.Command("go", "doc", arg)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go doc %s: %w", arg, err)
	}
	return string(out), nil
}

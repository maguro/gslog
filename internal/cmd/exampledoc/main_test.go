// Copyright the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"go/doc"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeModule writes a module with these packages to a temporary directory
// and returns the directory.  The root package and the package a have an
// example.  The package b has none.  The packages under internal, testdata,
// a hidden directory, and a nested module have an example that the walk must
// not find.  The docs directory has no Go files.
func writeModule(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	files := map[string]string{
		"go.mod":                     "module example.com/m\n\ngo 1.26\n",
		"m.go":                       "// Package m is the root package.\npackage m\n",
		"example_test.go":            "package m_test\n\nfunc Example() {}\n",
		"a/a.go":                     "// Package a has an example.\npackage a\n",
		"a/example_test.go":          "package a_test\n\nfunc ExampleFoo() {}\n",
		"b/b.go":                     "package b\n",
		"internal/c/c.go":            "package c\n",
		"internal/c/example_test.go": "package c_test\n\nfunc Example() {}\n",
		"testdata/d/d.go":            "package d\n",
		"testdata/d/example_test.go": "package d_test\n\nfunc Example() {}\n",
		".hidden/e/e.go":             "package e\n",
		".hidden/e/example_test.go":  "package e_test\n\nfunc Example() {}\n",
		"tools/go.mod":               "module example.com/m/tools\n\ngo 1.26\n",
		"tools/f.go":                 "package f\n",
		"tools/example_test.go":      "package f_test\n\nfunc Example() {}\n",
		"docs/README.md":             "no Go files\n",
	}

	for name, content := range files {
		path := filepath.Join(root, name)

		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	return root
}

func TestLoadPackages(t *testing.T) {
	root := writeModule(t)

	pkgs, err := loadPackages(root, "example.com/m")
	require.NoError(t, err)

	importPaths := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		importPaths = append(importPaths, p.importPath)
	}

	assert.Equal(t, []string{"example.com/m", "example.com/m/a"}, importPaths)

	require.Len(t, pkgs, 2)
	assert.Equal(t, "a", pkgs[1].name)
	assert.Equal(t, "Package a has an example.\n", pkgs[1].doc)
	require.Len(t, pkgs[1].examples, 1)
	assert.Equal(t, "Foo", pkgs[1].examples[0].Name)
}

func TestSkipDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"internal", true},
		{"testdata", true},
		{"vendor", true},
		{".git", true},
		{"_tools", true},
		{"stdout", false},
		{"internals", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, skipDir(tt.name))
		})
	}
}

func TestImportPath(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{".", "m4o.io/gslog"},
		{"stdout", "m4o.io/gslog/stdout"},
		{"./otel/detect", "m4o.io/gslog/otel/detect"},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			assert.Equal(t, tt.want, importPath("m4o.io/gslog", tt.dir))
		})
	}
}

func TestTitle(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"", "stdout"},
		{"_suffix", "stdout (suffix)"},
		{"_éclair", "stdout (éclair)"},
		{"NewHandler", "stdout.NewHandler"},
		{"NewHandler_withSourceAdded", "stdout.NewHandler (withSourceAdded)"},
		{"Handler_Handle", "stdout.Handler.Handle"},
		{"Handler_Handle_withLabels", "stdout.Handler.Handle (withLabels)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &doc.Example{Name: tt.name}

			assert.Equal(t, tt.want, title("stdout", ex))
		})
	}
}

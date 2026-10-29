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
	"bytes"
	"flag"
	"go/doc"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// moduleFile is the go.mod file of the modules that the tests write.
const moduleFile = "module example.com/m\n\ngo 1.26\n"

// writeModule writes a module with these packages to a temporary directory
// and returns the directory.  The root package and the package a have an
// example.  The package b has none.  The packages under internal, testdata,
// a hidden directory, and a nested module have an example that the walk must
// not find.  The docs directory has no Go files.
func writeModule(t *testing.T) string {
	t.Helper()

	files := map[string]string{
		"go.mod":                     moduleFile,
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

	return writeFiles(t, files)
}

// writeFiles writes the files to a temporary directory and returns the
// directory.  The keys of files are paths relative to the directory.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()

	for name, content := range files {
		path := filepath.Join(root, name)

		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}

	return root
}

// exampleModule is a module with one package that has one complete example.
var exampleModule = map[string]string{
	"go.mod": moduleFile,
	"m.go":   "// Package m is the root package.\npackage m\n",
	"example_test.go": `package m_test

import "fmt"

// Example prints a greeting.
func Example() {
	fmt.Println("hi")
	// Output: hi
}
`,
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

func TestLoadPackagesErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			"bad test file",
			map[string]string{
				"m.go":            "package m\n",
				"example_test.go": "not Go\n",
			},
		},
		{
			"bad test body",
			map[string]string{
				"m.go":            "package m\n",
				"example_test.go": "package m_test\n\nfunc Example() {\n",
			},
		},
		{
			"mixed packages",
			map[string]string{
				"m.go": "package m\n",
				"n.go": "package n\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeFiles(t, tt.files)

			_, err := loadPackages(root, "example.com/m")

			assert.Error(t, err)
		})
	}
}

func TestLoadPackagesMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")

	_, err := loadPackages(root, "example.com/m")

	assert.Error(t, err)
}

func TestModulePath(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"ok/go.mod":    "// comment\nmodule example.com/m \n\ngo 1.26\n",
		"empty/go.mod": "go 1.26\n",
	})

	okPath := filepath.Join(root, "ok", "go.mod")
	emptyPath := filepath.Join(root, "empty", "go.mod")
	missingPath := filepath.Join(root, "missing", "go.mod")

	module, err := modulePath(okPath)
	require.NoError(t, err)
	assert.Equal(t, "example.com/m", module)

	_, err = modulePath(emptyPath)
	assert.ErrorContains(t, err, "no module directive")

	_, err = modulePath(missingPath)
	assert.Error(t, err)
}

func TestCompare(t *testing.T) {
	root := writeFiles(t, map[string]string{"out.md": "text\n"})
	path := filepath.Join(root, "out.md")
	missing := filepath.Join(root, "missing.md")

	assert.NoError(t, compare(path, []byte("text\n")))
	assert.ErrorContains(t, compare(path, []byte("other\n")), "out of date")
	assert.Error(t, compare(missing, []byte("text\n")))
}

func TestMarkdown(t *testing.T) {
	got := markdown("Some text.\n\n\tcode()\n")

	assert.Contains(t, got, "Some text.")
	assert.Contains(t, got, "code()")
}

func TestRenderExample(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"go.mod": moduleFile,
		"m.go":   "package m\n",
		"example_test.go": `package m_test

import "fmt"

// Example_unordered prints in any order.
func Example_unordered() {
	fmt.Println("a")
	// Unordered output: a
}

func Example_silent() {
	fmt.Println("b")
}
`,
	})

	p, err := loadPackage(root, "example.com/m")
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Len(t, p.examples, 2)

	var buf bytes.Buffer

	for _, ex := range p.examples {
		require.NoError(t, renderExample(&buf, p.fset, p.name, ex))
	}

	got := buf.String()

	assert.Contains(t, got, "### m (unordered)")
	assert.Contains(t, got, "prints in any order.")
	assert.Contains(t, got, "Output, in any order:")
	assert.Contains(t, got, "### m (silent)")
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("```text")))
}

func TestRenderExampleNotComplete(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"go.mod": moduleFile,
		"m.go":   "package m\n",
		"example_test.go": `package m

import "fmt"

func Example() {
	fmt.Println("hi")
	// Output: hi
}
`,
	})

	p, err := loadPackage(root, "example.com/m")
	require.NoError(t, err)
	require.NotNil(t, p)

	var buf bytes.Buffer

	err = renderPackage(&buf, p)

	assert.ErrorContains(t, err, "is not a complete program")
}

func TestRender(t *testing.T) {
	root := writeFiles(t, exampleModule)

	text, err := render(root)
	require.NoError(t, err)

	got := string(text)

	assert.Contains(t, got, header)
	assert.Contains(t, got, "## example.com/m\n")
	assert.Contains(t, got, "Package m is the root package.")
	assert.Contains(t, got, "### m\n")
	assert.Contains(t, got, "fmt.Println(\"hi\")")
	assert.Contains(t, got, "Output:\n\n```text\nhi\n```\n")
}

func TestRenderErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			"no go.mod",
			map[string]string{},
			"go.mod",
		},
		{
			"bad package",
			map[string]string{
				"go.mod": moduleFile,
				"m.go":   "not Go\n",
			},
			"m.go",
		},
		{
			"example not complete",
			map[string]string{
				"go.mod":          moduleFile,
				"m.go":            "package m\n",
				"example_test.go": "package m\n\nfunc Example() {}\n",
			},
			"is not a complete program",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeFiles(t, tt.files)

			_, err := render(root)

			assert.ErrorContains(t, err, tt.want)
		})
	}
}

// runIn runs the command in dir with the arguments, and returns its error.
func runIn(t *testing.T, dir string, args ...string) error {
	t.Helper()

	t.Chdir(dir)

	oldArgs, oldFlags := os.Args, flag.CommandLine

	t.Cleanup(func() {
		os.Args, flag.CommandLine = oldArgs, oldFlags
	})

	os.Args = append([]string{"exampledoc"}, args...)
	flag.CommandLine = flag.NewFlagSet("exampledoc", flag.ContinueOnError)

	return run()
}

func TestRun(t *testing.T) {
	root := writeFiles(t, exampleModule)
	out := filepath.Join(root, "examples.md")

	require.NoError(t, runIn(t, root, "-o", out))

	written, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Contains(t, string(written), "## example.com/m")

	assert.NoError(t, runIn(t, root, "-check", "-o", out))

	require.NoError(t, os.WriteFile(out, []byte("old\n"), 0o600))

	assert.ErrorContains(t, runIn(t, root, "-check", "-o", out), "out of date")
	assert.ErrorContains(t, runIn(t, root, "extra"), "no arguments")

	empty := t.TempDir()

	assert.Error(t, runIn(t, empty))
}

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type corpus struct {
	label   string
	content string
}

func main() {
	sourceDir := flag.String("source", "../vef-framework-go", "path to the VEF Framework Go source repository")
	outDir := flag.String("out", ".", "path to the VEF Framework Go docs repository")
	flag.Parse()

	sourceRoot := cleanAbs(*sourceDir)
	docsRoot := cleanAbs(*outDir)

	jsSource := readCorpus("js package source", filepath.Join(sourceRoot, "js/js.go"))
	stdlibSource := readCorpus("js stdlib source", filepath.Join(sourceRoot, "js/stdlib.go"))
	libSource := readCorpus("js lib source", filepath.Join(sourceRoot, "js/lib.go"))
	englishDocs := readCorpus("English JS docs", filepath.Join(docsRoot, "docs/data-tools/js-engine.md"))
	chineseDocs := readCorpus("Chinese JS docs", filepath.Join(docsRoot, "i18n/zh-Hans/docusaurus-plugin-content-docs/current/data-tools/js-engine.md"))
	docs := []corpus{englishDocs, chineseDocs}

	checks := []struct {
		corpus corpus
		terms  []string
	}{
		{
			corpus: jsSource,
			terms: []string{
				"Value      = goja.Value",
				"Object     = goja.Object",
				"Program    = goja.Program",
				"AstProgram = ast.Program",
				"Compile     = goja.Compile",
				"MustCompile = goja.MustCompile",
				"IsNaN       = goja.IsNaN",
				"IsString    = goja.IsString",
				"IsBigInt    = goja.IsBigInt",
				"IsNumber    = goja.IsNumber",
				"IsInfinity  = goja.IsInfinity",
				"IsUndefined = goja.IsUndefined",
				"IsNull      = goja.IsNull",
				"return goja.Parse(name, src, parser.WithDisableSourceMaps)",
			},
		},
		{
			corpus: stdlibSource,
			terms: []string{
				"//go:embed libs/stdlib.bundle.js",
				"var stdLibs = []Lib{",
				"ProgramLib(\"stdlib\"",
				"MustCompile(\"stdlib\"",
			},
		},
		{
			corpus: libSource,
			terms: []string{
				"type Lib interface {",
				"Name() string",
				"Install(rt *Runtime) error",
				"func ProgramLib(name string, program *Program) Lib",
				"func SourceLib(name, source string) (Lib, error)",
			},
		},
	}

	docTerms := []string{
		"goja",
		"goja.TagFieldNameMapper",
		"dayjs",
		"BigNumber",
		"fxp",
		"radashi",
		"z",
		"js.Compile",
		"js.MustCompile",
		"js.IsNaN",
		"js.IsString",
		"js.IsBigInt",
		"js.IsNumber",
		"js.IsInfinity",
		"js.IsUndefined",
		"js.IsNull",
		"js.Parse",
		"js.ProgramLib",
		"js.SourceLib",
		"js.Runtime",
		"js.Value",
		"js.Object",
		"js.Program",
		"js.AstProgram",
		"js.NewEngine",
		"js.WithBaseLibs",
		"js.WithLibs",
		"js.WithoutStdLibs",
		"js.EnableLibs",
		"js.WithRunTimeout",
		"js.WithMaxCallStackSize",
		"public API index",
	}
	englishDocTerms := []string{
		"vendored esbuild bundle",
		"ecosystem-native global name",
		"bignumber.js",
		"fast-xml-parser",
		"core-js polyfills",
		"WHATWG URL",
		"Zod",
		"Engine / Runtime / Lib",
		"safe for concurrent use",
	}
	chineseDocTerms := []string{
		"vendored esbuild bundle",
		"生态原生的",
		"Engine / Runtime / Lib",
		"可并发使用",
		"不",
		"bignumber.js",
		"fast-xml-parser",
		"core-js polyfill",
		"WHATWG URL",
		"Zod",
	}

	var failures []string
	for _, check := range checks {
		failures = append(failures, missingTerms(check.corpus, check.terms)...)
	}

	for _, doc := range docs {
		failures = append(failures, missingTerms(doc, docTerms)...)
	}
	failures = append(failures, missingTerms(englishDocs, englishDocTerms)...)
	failures = append(failures, missingTerms(chineseDocs, chineseDocTerms)...)
	failures = append(failures, runGoTest(sourceRoot)...)

	sort.Strings(failures)
	if len(failures) > 0 {
		panic(fmt.Errorf("JS contract verification failed:\n%s", strings.Join(failures, "\n")))
	}

	fmt.Println("JS contract docs verified: 3 source files, 2 doc mirrors, go test ./js")
}

func readCorpus(label, path string) corpus {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("failed to read %s at %s: %w", label, path, err))
	}

	return corpus{label: label, content: string(content)}
}

func missingTerms(c corpus, terms []string) []string {
	var failures []string
	for _, term := range terms {
		if !containsTerm(c.content, term) {
			failures = append(failures, fmt.Sprintf("%s missing term: %s", c.label, term))
		}
	}

	return failures
}

func containsTerm(content, term string) bool {
	if strings.Contains(content, term) {
		return true
	}

	return strings.Contains(normalizeWhitespace(content), normalizeWhitespace(term))
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func runGoTest(sourceRoot string) []string {
	cmd := exec.Command("go", "test", "./js")
	cmd.Dir = sourceRoot
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return []string{fmt.Sprintf("go test ./js failed: %v\n%s", err, strings.TrimSpace(output.String()))}
	}

	return nil
}

func cleanAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}

	return filepath.Clean(abs)
}

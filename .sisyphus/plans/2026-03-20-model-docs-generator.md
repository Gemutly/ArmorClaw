# Model Documentation Generator Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a CLI tool that generates model documentation from the embedded provider registry with optional Catwalk enrichment.

**Architecture:** Reuse existing provider registry and Catwalk client. Extract `fallbackModels` map to shared location. CLI loads providers, merges models, renders markdown template, writes to `docs/reference/models.md`.

**Tech Stack:** Go 1.24, existing `pkg/providers` and `internal/wizard/catwalk` packages

---

## File Structure

| File | Responsibility |
|------|----------------|
| `bridge/pkg/providers/models.go` | Shared `fallbackModels` map (extracted from quick.go) |
| `bridge/cmd/gen-models/main.go` | CLI entry point, flag parsing, orchestration |
| `bridge/cmd/gen-models/generator.go` | Core generation logic (load, merge, render) |
| `bridge/cmd/gen-models/generator_test.go` | Unit tests for generator |
| `docs/reference/models.md` | Generated output (created by tool) |
| `.github/workflows/docs.yml` | CI workflow for doc verification |
| `README.md` | Add link to models.md |
| `bridge/internal/wizard/quick.go` | Import shared models (remove duplicate) |

---

## Chunk 1: Extract Shared Models

### Task 1: Create shared models package

**Files:**
- Create: `bridge/pkg/providers/models.go`

- [ ] **Step 1: Create models.go with extracted fallbackModels**

```go
// Package providers defines the embedded provider registry and fallback model lists.
package providers

// FallbackModels provides static model lists when Catwalk is unavailable.
// Each key is a provider ID, value is list of model IDs.
var FallbackModels = map[string][]ModelInfo{
	"openai": {
		{ID: "gpt-4.1", Name: "GPT-4.1", ContextSize: 0},
		{ID: "gpt-4o", Name: "GPT-4o", ContextSize: 128000},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ContextSize: 128000},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ContextSize: 128000},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ContextSize: 16385},
	},
	"anthropic": {
		{ID: "claude-3-opus", Name: "Claude 3 Opus", ContextSize: 200000},
		{ID: "claude-3-sonnet", Name: "Claude 3 Sonnet", ContextSize: 200000},
		{ID: "claude-3-haiku", Name: "Claude 3 Haiku", ContextSize: 200000},
	},
	"google": {
		{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", ContextSize: 1000000},
		{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", ContextSize: 1000000},
		{ID: "gemini-pro", Name: "Gemini Pro", ContextSize: 32760},
	},
	"xai": {
		{ID: "grok-2", Name: "Grok 2", ContextSize: 131072},
		{ID: "grok-2-vision", Name: "Grok 2 Vision", ContextSize: 32768},
		{ID: "grok-beta", Name: "Grok Beta", ContextSize: 131072},
	},
	"zhipu": {
		{ID: "glm-4", Name: "GLM-4", ContextSize: 128000},
		{ID: "glm-4v", Name: "GLM-4V", ContextSize: 128000},
		{ID: "glm-4-flash", Name: "GLM-4 Flash", ContextSize: 128000},
		{ID: "glm-3-turbo", Name: "GLM-3 Turbo", ContextSize: 128000},
	},
	"deepseek": {
		{ID: "deepseek-chat", Name: "DeepSeek Chat", ContextSize: 64000},
		{ID: "deepseek-coder", Name: "DeepSeek Coder", ContextSize: 64000},
	},
	"moonshot": {
		{ID: "moonshot-v1-8k", Name: "Moonshot V1 8K", ContextSize: 8192},
		{ID: "moonshot-v1-32k", Name: "Moonshot V1 32K", ContextSize: 32768},
		{ID: "moonshot-v1-128k", Name: "Moonshot V1 128K", ContextSize: 131072},
	},
	"groq": {
		{ID: "llama-3.1-70b-versatile", Name: "Llama 3.1 70B Versatile", ContextSize: 131072},
		{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B Instant", ContextSize: 131072},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", ContextSize: 32768},
	},
	"openrouter": {
		{ID: "openai/gpt-4o", Name: "OpenAI GPT-4o", ContextSize: 128000},
		{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", ContextSize: 200000},
		{ID: "google/gemini-pro-1.5", Name: "Gemini Pro 1.5", ContextSize: 1000000},
	},
	"nvidia": {
		{ID: "meta/llama-3.1-405b-instruct", Name: "Llama 3.1 405B", ContextSize: 128000},
		{ID: "meta/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", ContextSize: 128000},
	},
	"cloudflare": {
		{ID: "@cf/meta/llama-3.1-8b-instruct", Name: "Llama 3.1 8B", ContextSize: 8192},
	},
	"ollama": {
		{ID: "llama3.2", Name: "Llama 3.2", ContextSize: 128000},
		{ID: "llama3.1", Name: "Llama 3.1", ContextSize: 128000},
		{ID: "mistral", Name: "Mistral", ContextSize: 32768},
	},
}

// ModelInfo represents a model's metadata.
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContextSize int    `json:"context_size,omitempty"`
}

// GetModels returns models for a provider, or empty slice if not found.
func GetModels(providerID string) []ModelInfo {
	return FallbackModels[providerID]
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go build ./pkg/providers/...`
Expected: Success (no output)

- [ ] **Step 3: Commit**

```bash
git add bridge/pkg/providers/models.go
git commit -m "feat(providers): extract fallbackModels to shared package"
```

### Task 2: Update quick.go to use shared models

**Files:**
- Modify: `bridge/internal/wizard/quick.go`

- [ ] **Step 1: Replace inline fallbackModels with import**

Remove lines 22-33 (the `fallbackModels` var declaration) and update `getModelOptions`:

```go
// In imports, add:
"github.com/armorclaw/bridge/pkg/providers"

// In getModelOptions, replace fallbackModels[providerKey] with:
if models := providers.GetModels(providerKey); len(models) > 0 {
    fmt.Println("  [Wizard] Using default model list for provider:", providerKey)
    result := make([]string, len(models))
    for i, m := range models {
        result[i] = m.ID
    }
    return result
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go build ./internal/wizard/...`
Expected: Success (no output)

- [ ] **Step 3: Run existing wizard tests**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go test ./internal/wizard/... -v`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add bridge/internal/wizard/quick.go
git commit -m "refactor(wizard): use shared fallbackModels from providers package"
```

---

## Chunk 2: Build Generator CLI

### Task 3: Create generator package

**Files:**
- Create: `bridge/cmd/gen-models/generator.go`
- Create: `bridge/cmd/gen-models/generator_test.go`

- [ ] **Step 1: Write the generator interface and tests first**

```go
// bridge/cmd/gen-models/generator.go
package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/armorclaw/bridge/internal/wizard"
	"github.com/armorclaw/bridge/pkg/providers"
)

// Options configures the generator.
type Options struct {
	OutputPath string // Where to write the markdown file
	CatwalkURL string // Optional Catwalk URL for enrichment
}

// ProviderDoc represents a provider with its models for documentation.
type ProviderDoc struct {
	ID       string
	Name     string
	Protocol string
	Aliases  []string
	Models   []ModelDoc
}

// ModelDoc represents a model for documentation.
type ModelDoc struct {
	ID          string
	Name        string
	ContextSize int
}

// Generator creates model documentation.
type Generator struct {
	opts Options
}

// NewGenerator creates a new generator.
func NewGenerator(opts Options) *Generator {
	return &Generator{opts: opts}
}

// Generate creates the markdown documentation.
func (g *Generator) Generate() (string, error) {
	docs := g.loadProviders()

	// Optional Catwalk enrichment
	if g.opts.CatwalkURL != "" {
		g.enrichFromCatwalk(docs)
	}

	return g.renderMarkdown(docs)
}

func (g *Generator) loadProviders() []*ProviderDoc {
	registry := providers.LoadEmbeddedRegistry()
	docs := make([]*ProviderDoc, 0, registry.GetProviderCount())

	for _, p := range registry.Providers {
		doc := &ProviderDoc{
			ID:       p.ID,
			Name:     p.Name,
			Protocol: p.Protocol,
			Aliases:  p.Aliases,
			Models:   make([]ModelDoc, 0),
		}

		// Load fallback models
		if models := providers.GetModels(p.ID); len(models) > 0 {
			for _, m := range models {
				doc.Models = append(doc.Models, ModelDoc{
					ID:          m.ID,
					Name:        m.Name,
					ContextSize: m.ContextSize,
				})
			}
		}

		docs = append(docs, doc)
	}

	return docs
}

func (g *Generator) enrichFromCatwalk(docs []*ProviderDoc) {
	client := wizard.NewCatwalkClient(g.opts.CatwalkURL)
	if !client.IsAvailable() {
		return
	}

	catwalkProviders, err := client.FetchProviders()
	if err != nil {
		return
	}

	// Build lookup map
	docMap := make(map[string]*ProviderDoc)
	for _, d := range docs {
		docMap[d.ID] = d
	}

	// Enrich with Catwalk data
	for _, cp := range catwalkProviders {
		if doc, ok := docMap[cp.ID]; ok && len(cp.Models) > 0 {
			// Replace models with Catwalk data (richer)
			doc.Models = make([]ModelDoc, 0, len(cp.Models))
			for _, m := range cp.Models {
				doc.Models = append(doc.Models, ModelDoc{
					ID:          m.ID,
					Name:        m.Name,
					ContextSize: m.ContextSize,
				})
			}
		}
	}
}

func (g *Generator) renderMarkdown(docs []*ProviderDoc) (string, error) {
	tmpl := `# AI Provider Models

> Generated from embedded provider registry.
> Last updated: {{.Timestamp}}

## Providers

| ID | Name | Protocol | Aliases |
|----|------|----------|---------|
{{- range .Providers}}
| {{.ID}} | {{.Name}} | {{.Protocol}} | {{aliasList .Aliases}} |
{{- end}}

## Models by Provider

{{- range .Providers}}
{{- if .Models}}

### {{.Name}}{{if .Aliases}} ({{join .Aliases ", "}}){{end}}

| Model ID | Display Name | Context |
|----------|--------------|---------|
{{- range .Models}}
| {{.ID}} | {{.Name}} | {{contextSize .ContextSize}} |
{{- end}}
{{- end}}
{{- end}}
`

	funcs := template.FuncMap{
		"aliasList": func(aliases []string) string {
			if len(aliases) == 0 {
				return "-"
			}
			return strings.Join(aliases, ", ")
		},
		"join": strings.Join,
		"contextSize": func(size int) string {
			if size == 0 {
				return "-"
			}
			return fmt.Sprintf("%d", size)
		},
	}

	t, err := template.New("models").Funcs(funcs).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	data := struct {
		Timestamp string
		Providers []*ProviderDoc
	}{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Providers: docs,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}
```

- [ ] **Step 2: Write unit tests**

```go
// bridge/cmd/gen-models/generator_test.go
package main

import (
	"strings"
	"testing"
)

func TestGenerator_LoadProviders(t *testing.T) {
	gen := NewGenerator(Options{})
	docs := gen.loadProviders()

	if len(docs) == 0 {
		t.Error("expected at least one provider")
	}

	// Check OpenAI exists
	found := false
	for _, d := range docs {
		if d.ID == "openai" {
			found = true
			if len(d.Models) == 0 {
				t.Error("openai should have models")
			}
			break
		}
	}
	if !found {
		t.Error("openai provider not found")
	}
}

func TestGenerator_RenderMarkdown(t *testing.T) {
	gen := NewGenerator(Options{})
	docs := []*ProviderDoc{
		{
			ID:       "test",
			Name:     "Test Provider",
			Protocol: "openai",
			Aliases:  []string{"t", "testing"},
			Models: []ModelDoc{
				{ID: "model-1", Name: "Model 1", ContextSize: 1000},
			},
		},
	}

	md, err := gen.renderMarkdown(docs)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if !strings.Contains(md, "# AI Provider Models") {
		t.Error("missing header")
	}
	if !strings.Contains(md, "Test Provider") {
		t.Error("missing provider name")
	}
	if !strings.Contains(md, "model-1") {
		t.Error("missing model ID")
	}
	if !strings.Contains(md, "t, testing") {
		t.Error("missing aliases")
	}
}

func TestGenerator_CatwalkEnrichment_SkippedWhenUnavailable(t *testing.T) {
	gen := NewGenerator(Options{
		CatwalkURL: "http://localhost:9999", // Unreachable
	})

	docs := gen.loadProviders()
	originalCount := len(docs[0].Models)

	gen.enrichFromCatwalk(docs)

	// Should not change models when Catwalk unavailable
	if len(docs[0].Models) != originalCount {
		t.Error("models changed despite Catwalk being unavailable")
	}
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go test ./cmd/gen-models/... -v`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add bridge/cmd/gen-models/generator.go bridge/cmd/gen-models/generator_test.go
git commit -m "feat(gen-models): add generator package with tests"
```

### Task 4: Create CLI main entry point

**Files:**
- Create: `bridge/cmd/gen-models/main.go`

- [ ] **Step 1: Write the CLI entry point**

```go
// bridge/cmd/gen-models/main.go
// Command gen-models generates model documentation from the provider registry.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"log/slog"
)

func main() {
	outputPath := flag.String("output", "", "Output file path (default: docs/reference/models.md relative to repo root)")
	catwalkURL := flag.String("catwalk-url", "", "Optional Catwalk URL for enrichment (default: none)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	if *verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	// Determine output path
	out := *outputPath
	if out == "" {
		// Find repo root (go up from bridge/cmd/gen-models)
		repoRoot, err := findRepoRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		out = filepath.Join(repoRoot, "docs", "reference", "models.md")
	}

	opts := Options{
		OutputPath: out,
		CatwalkURL: *catwalkURL,
	}

	gen := NewGenerator(opts)
	md, err := gen.Generate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating docs: %v\n", err)
		os.Exit(1)
	}

	// Ensure directory exists
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Write file
	if err := os.WriteFile(out, []byte(md), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated: %s\n", out)
}

func findRepoRoot() (string, error) {
	// Start from current directory and go up looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found go.mod, go up one more level to repo root
			return filepath.Dir(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod")
		}
		dir = parent
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go build ./cmd/gen-models/...`
Expected: Success (no output)

- [ ] **Step 3: Run the generator**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go run ./cmd/gen-models --verbose`
Expected: "Generated: /home/mink/src/armorclaw-omo/docs/reference/models.md"

- [ ] **Step 4: Verify output exists**

Run: `head -30 /home/mink/src/armorclaw-omo/docs/reference/models.md`
Expected: Header with "AI Provider Models" and provider table

- [ ] **Step 5: Commit**

```bash
git add bridge/cmd/gen-models/main.go docs/reference/models.md
git commit -m "feat(gen-models): add CLI tool and generate initial docs"
```

---

## Chunk 3: CI Integration and README

### Task 5: Create docs workflow

**Files:**
- Create: `.github/workflows/docs.yml`

- [ ] **Step 1: Write the workflow**

```yaml
# Documentation verification workflow
# Runs on changes to provider/model source files

name: Documentation

on:
  push:
    branches: [main]
    paths:
      - 'configs/providers.json'
      - 'bridge/pkg/providers/**/*.go'
      - 'bridge/internal/wizard/quick.go'
      - 'bridge/internal/wizard/catwalk.go'
      - 'bridge/cmd/gen-models/**'
  pull_request:
    paths:
      - 'configs/providers.json'
      - 'bridge/pkg/providers/**/*.go'
      - 'bridge/internal/wizard/quick.go'
      - 'bridge/internal/wizard/catwalk.go'
      - 'bridge/cmd/gen-models/**'
  workflow_dispatch:

jobs:
  verify-model-docs:
    name: Verify Model Docs
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: 'bridge/go.mod'
          cache: true

      - name: Generate model docs
        run: |
          cd bridge
          go run ./cmd/gen-models --output ../docs/reference/models.md

      - name: Check for drift
        run: |
          if git diff --exit-code docs/reference/models.md; then
            echo "Model docs up to date"
          else
            echo "::warning::Model docs out of date. Run: cd bridge && go run ./cmd/gen-models"
            echo ""
            echo "Diff:"
            git diff docs/reference/models.md
          fi
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/docs.yml
git commit -m "ci(docs): add workflow to verify model docs"
```

### Task 6: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add link in Supported AI Providers section**

Find the "Supported AI Providers" section (around line 180-220) and add after the intro paragraph:

```markdown
### Supported AI Providers (v4.5.0+)

> **Full model list**: See [docs/reference/models.md](docs/reference/models.md) for all available models per provider.

ArmorClaw uses a Provider Registry pattern...
```

- [ ] **Step 2: Verify the link works**

Run: `grep -n "docs/reference/models.md" /home/mink/src/armorclaw-omo/README.md`
Expected: One match in the Supported AI Providers section

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add link to generated model docs"
```

---

## Chunk 4: Final Verification

### Task 7: Run full test suite

- [ ] **Step 1: Run all bridge tests**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go test ./... -v`
Expected: All tests pass

- [ ] **Step 2: Run generator one more time**

Run: `cd /home/mink/src/armorclaw-omo/bridge && go run ./cmd/gen-models`
Expected: "Generated: /home/mink/src/armorclaw-omo/docs/reference/models.md"

- [ ] **Step 3: Verify no git changes**

Run: `git status`
Expected: "nothing to commit, working tree clean" (or only unrelated changes)

### Task 8: Push changes

- [ ] **Step 1: Push to remote**

```bash
git push origin main
```

- [ ] **Step 2: Verify CI workflow runs**

Check GitHub Actions for the "Documentation" workflow.

---

## Summary

| Component | Files | Status |
|-----------|-------|--------|
| Shared models | `bridge/pkg/providers/models.go` | - [ ] |
| Generator package | `bridge/cmd/gen-models/generator.go` | - [ ] |
| Generator tests | `bridge/cmd/gen-models/generator_test.go` | - [ ] |
| CLI entry point | `bridge/cmd/gen-models/main.go` | - [ ] |
| Generated docs | `docs/reference/models.md` | - [ ] |
| CI workflow | `.github/workflows/docs.yml` | - [ ] |
| README link | `README.md` | - [ ] |
| Quick.go refactor | `bridge/internal/wizard/quick.go` | - [ ] |

package scl

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestParseFile_IRSEVContract(t *testing.T) {
	contractPath := filepath.Clean("../contracts/IRSEV_CONTRACT.md")
	contract, err := ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}

	requiredSections := []string{
		"ISSUE",
		"ROOT_CAUSE",
		"SOLUTION",
		"EXECUTION",
		"VALIDATION",
	}

	for _, sectionName := range requiredSections {
		items, ok := contract.Sections[sectionName]
		if !ok {
			t.Fatalf("missing required section %q", sectionName)
		}
		if len(items) == 0 {
			t.Fatalf("section %q must contain at least one item", sectionName)
		}
	}

	for sectionName, items := range contract.Sections {
		for i, item := range items {
			if strings.Contains(item, "<!-- CONSTANTS:$(") || strings.Contains(item, "$$(") {
				t.Fatalf("section %q item %d contains unresolved constant token: %q", sectionName, i, item)
			}
		}
	}

	issueItems := contract.Sections["ISSUE"]
	if len(issueItems) < 2 {
		t.Fatalf("ISSUE section must have at least 2 items for order assertion")
	}
	if issueItems[0] != "Describe the objective, change request, or observed problem." {
		t.Fatalf("unexpected ISSUE[0]: %q", issueItems[0])
	}
	if issueItems[1] != "Use concrete examples when applicable (before → after)." {
		t.Fatalf("unexpected ISSUE[1]: %q", issueItems[1])
	}

	if len(contract.OrderedConstants) < 2 {
		t.Fatalf("expected at least 2 ordered constants, got %d", len(contract.OrderedConstants))
	}
	if contract.OrderedConstants[0].Key != "SCOPE_CORE" {
		t.Fatalf("unexpected first constant key: %q", contract.OrderedConstants[0].Key)
	}
	if contract.OrderedConstants[1].Key != "GUARDRAIL_NO_REWRITE" {
		t.Fatalf("unexpected second constant key: %q", contract.OrderedConstants[1].Key)
	}

	expectedSectionOrder := []string{"ISSUE", "ROOT_CAUSE", "SOLUTION", "EXECUTION", "VALIDATION"}
	if len(contract.OrderedSections) < len(expectedSectionOrder) {
		t.Fatalf("expected at least %d ordered sections, got %d", len(expectedSectionOrder), len(contract.OrderedSections))
	}
	for i, name := range expectedSectionOrder {
		if contract.OrderedSections[i].Name != name {
			t.Fatalf("unexpected section order at index %d: got %q, want %q", i, contract.OrderedSections[i].Name, name)
		}
		if len(contract.OrderedSections[i].Routes) == 0 {
			t.Fatalf("expected routed blocks for section %q", contract.OrderedSections[i].Name)
		}
	}

	rendered, err := json.Marshal(contract.RenderView())
	if err != nil {
		t.Fatalf("marshal render view failed: %v", err)
	}
	renderedStr := string(rendered)
	if strings.Contains(renderedStr, "<!-- CONSTANTS:$(") || strings.Contains(renderedStr, "$$(") {
		t.Fatalf("render output contains unresolved constant token: %s", renderedStr)
	}

	templateText := contract.GoTemplate()
	if strings.TrimSpace(templateText) == "" {
		t.Fatalf("GoTemplate returned empty template")
	}
	if templateText != contract.GoTemplate() {
		t.Fatalf("GoTemplate must be deterministic across calls")
	}
	if !strings.Contains(templateText, "{{define \"route\"}}") {
		t.Fatalf("template missing recursive route definition")
	}
	if !strings.Contains(templateText, "## {{ .Name }}") {
		t.Fatalf("template missing generic section heading")
	}

	tmpl, err := template.New("contract").Parse(templateText)
	if err != nil {
		t.Fatalf("template parse failed: %v", err)
	}

	tplView := contract.TemplateView()
	if len(tplView.Constants) != len(contract.OrderedConstants) {
		t.Fatalf("template constants size mismatch: got %d want %d", len(tplView.Constants), len(contract.OrderedConstants))
	}
	if len(tplView.Sections) != len(contract.OrderedSections) {
		t.Fatalf("template sections size mismatch: got %d want %d", len(tplView.Sections), len(contract.OrderedSections))
	}
	for i := range tplView.Constants {
		if strings.TrimSpace(tplView.Constants[i].Symbol) == "" {
			t.Fatalf("constant symbol at index %d is empty", i)
		}
	}
	for i := range tplView.Sections {
		if strings.TrimSpace(tplView.Sections[i].Symbol) == "" {
			t.Fatalf("section symbol at index %d is empty", i)
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tplView); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}

	renderedTemplate := buf.String()
	if strings.Contains(renderedTemplate, "<!-- CONSTANTS:$(") || strings.Contains(renderedTemplate, "$$(") {
		t.Fatalf("template execute output contains unresolved constant token: %s", renderedTemplate)
	}

	expectedRenderedSectionOrder := []string{"## ISSUE", "## ROOT_CAUSE", "## SOLUTION", "## EXECUTION", "## VALIDATION"}
	lastIdx := -1
	for _, sectionHeader := range expectedRenderedSectionOrder {
		idx := strings.Index(renderedTemplate, sectionHeader)
		if idx < 0 {
			t.Fatalf("rendered template missing section header %q", sectionHeader)
		}
		if idx <= lastIdx {
			t.Fatalf("rendered template section order invalid around %q", sectionHeader)
		}
		lastIdx = idx
	}
}

func TestParseFile_NestedRoutes(t *testing.T) {
	content := `
<!-- CONSTANTS:START -->
<pre>
SCOPE_CORE = "scope"
</pre>
<!-- CONSTANTS:END -->

<!-- $${ -->
## EXECUTION
<!-- $$[.steps -->
- step a
- step b
<!-- $$] -->
<!-- $$[.failure_modes -->
- failure a
<!-- $$] -->
<!-- $$[.block -->
- block item
<!-- $$[.nested_block -->
- nested one
<!-- $$] -->
<!-- $$] -->
<!-- $$} -->
`

	path := writeTempContract(t, content)
	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile nested routes failed: %v", err)
	}

	if len(contract.OrderedSections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(contract.OrderedSections))
	}
	sec := contract.OrderedSections[0]
	if sec.Name != "EXECUTION" {
		t.Fatalf("unexpected section name: %q", sec.Name)
	}
	if len(sec.Routes) != 3 {
		t.Fatalf("expected 3 top-level routes, got %d", len(sec.Routes))
	}
	if sec.Routes[0].Term != "steps" || sec.Routes[1].Term != "failure_modes" || sec.Routes[2].Term != "block" {
		t.Fatalf("unexpected top route order: %+v", sec.Routes)
	}
	if len(sec.Routes[2].Children) != 1 || sec.Routes[2].Children[0].Term != "nested_block" {
		t.Fatalf("unexpected nested route tree under block: %+v", sec.Routes[2].Children)
	}

	routesBySection, ok := contract.SectionRoutes["execution"]
	if !ok {
		t.Fatalf("expected section routes for execution")
	}
	expectedPaths := []string{"execution.steps", "execution.failure_modes", "execution.block", "execution.block.nested_block"}
	for _, p := range expectedPaths {
		if _, ok := routesBySection[p]; !ok {
			t.Fatalf("missing canonical route path %q", p)
		}
	}

	if got := contract.Sections["EXECUTION"]; len(got) == 0 || got[0] != "step a" {
		t.Fatalf("Sections compatibility mapping should map EXECUTION to first routed block items, got %v", got)
	}

	tpl := contract.GoTemplate()
	tplExec, err := template.New("contract").Parse(tpl)
	if err != nil {
		t.Fatalf("template parse failed: %v", err)
	}
	var out bytes.Buffer
	if err := tplExec.Execute(&out, contract.TemplateView()); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "<!-- $$[.steps -->") {
		t.Fatalf("rendered template missing .steps route block: %s", rendered)
	}
	if !strings.Contains(rendered, "<!-- $$[.nested_block -->") {
		t.Fatalf("rendered template missing nested route block: %s", rendered)
	}
}

func TestParseFile_StrictFailures(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		errContains string
	}{
		{
			name: "legacy list token unsupported",
			content: `
<!-- CONSTANTS:START -->
<pre>
SCOPE_CORE = "scope"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## ISSUE
<!-- $$[ -->
- item
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "legacy list token",
		},
		{
			name: "malformed constants block",
			content: `
<!-- CONSTANTS:START -->
<pre>
INVALID_LINE
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## ISSUE
<!-- $$[ -->
- item
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "invalid constant line",
		},
		{
			name: "malformed section token order",
			content: `
<!-- CONSTANTS:START -->
<pre>
K = "v"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## ISSUE
<!-- $$] -->
<!-- $$[.description -->
- item
<!-- $$} -->
`,
			errContains: "unexpected list end token",
		},
		{
			name: "invalid list line",
			content: `
<!-- CONSTANTS:START -->
<pre>
K = "v"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## ISSUE
<!-- $$[.description -->
not-a-list-item
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "invalid token",
		},
		{
			name: "undefined constant reference",
			content: `
<!-- CONSTANTS:START -->
<pre>
KNOWN = "ok"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## ISSUE
<!-- $$[.description -->
- <!-- CONSTANTS:$(MISSING) -->
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "undefined constant key",
		},
		{
			name: "invalid dotted single token route",
			content: `
<!-- CONSTANTS:START -->
<pre>
K = "v"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## EXECUTION
<!-- $$[.block.nested_block -->
- item
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "single-segment route terms",
		},
		{
			name: "invalid hyphen route term",
			content: `
<!-- CONSTANTS:START -->
<pre>
K = "v"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## EXECUTION
<!-- $$[.failure-modes -->
- item
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "hyphen '-' is not allowed",
		},
		{
			name: "unclosed nested route blocks",
			content: `
<!-- CONSTANTS:START -->
<pre>
K = "v"
</pre>
<!-- CONSTANTS:END -->
<!-- $${ -->
## EXECUTION
<!-- $$[.block -->
- item
<!-- $$[.nested_block -->
- nested
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "closed before all list blocks were closed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempContract(t, tc.content)
			_, err := ParseFile(path)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %q", tc.errContains, err.Error())
			}
		})
	}
}

func writeTempContract(t *testing.T, content string) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "scl-contract-*.md")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	return file.Name()
}

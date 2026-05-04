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
	if !strings.Contains(templateText, "{{- range .Constants }}") {
		t.Fatalf("template missing constants range")
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

func TestParseFile_StrictFailures(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		errContains string
	}{
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
<!-- $$[ -->
- item
<!-- $$} -->
`,
			errContains: "expected list start token",
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
<!-- $$[ -->
not-a-list-item
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "invalid list entry",
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
<!-- $$[ -->
- <!-- CONSTANTS:$(MISSING) -->
<!-- $$] -->
<!-- $$} -->
`,
			errContains: "undefined constant key",
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

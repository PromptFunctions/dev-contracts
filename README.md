# dev-contracts

Structured contract definitions and parser utilities for deterministic agent workflows.

## What This Repo Contains

- `contracts/`: Markdown contract files (SCL-compatible contract sources).
  - `contracts/IRSEV_CONTRACT.md`
  - `contracts/DELIVERY_CONTRACT_EXPANDED.md`
- `scl/`: Go SCL parser module.
  - `scl/structured_contract.go`
  - `scl/structured_contract_test.go`
  - `scl/print_contract.go` (tiny runner to print parsed contract)
- `file.md`: local working draft/example content.

## What Is SCL (Structured Contract Language)

SCL is a minimal DSL embedded in Markdown for deterministic contracts.

- Humans can write/read the contract as normal Markdown.
- Agents can parse it with strict structure rules.
- The same source can drive both machine processing and final rendered contract output.

SCL primitives in this repo:
- constants block (`<!-- CONSTANTS:START --> ... <!-- CONSTANTS:END -->`)
- section boundaries (`<!-- $${ --> ... <!-- $$} -->`)
- routed list boundaries (`<!-- $$[.term --> ... <!-- $$] -->`)
- list entries (`- ...`)
- constant references (`$$(KEY)` and `<!-- CONSTANTS:$(KEY) -->`)

## Contract-Based Workflow

Core pipeline:

`SCL contract input -> DSL parser (scl/structured_contract.go) -> returns BOTH structured data and a Go template`

### Role of the structured contract (struct)

Use the parsed struct as the reference schema/shape for structured outputs in your LLM call.
This applies regardless of provider (Anthropic/OpenAI/others): the key is structured outputs compatibility.

### Role of the generated template

Use the generated template after your structured-output JSON is returned.
The template renders the final contract text that the agent sends back to the user (human or agent).

Template usage is optional because JSON is already structured.
However, rendering from returned JSON is the preferred contract-workflow pattern because it avoids accidental post-response mutation of contract values.

## Working With Prompt Control

`dev-contracts` and `PromptControl` can be used independently, but they are designed to compose well.

### `PromptControl` role

- Enforces deterministic key-level contract compliance on returned LLM JSON.
- Uses Bloom-filter pre-check + exact-map verification to detect missing keys.
- Returns explicit completion/missing status for enforcement loops.

### Combined workflow (recommended)

1. SCL contract input.
2. Parse via `scl/structured_contract.go` into mapped contract data.
3. Use that mapped contract data as the structured-output reference in your LLM call.
4. Validate each returned JSON with Prompt Control to enforce exact contract structure.
5. Re-prompt on missing keys until complete.
6. Render final contract text from the returned JSON via generated template (preferred, mutation-safe path).

Practical interpretation:

- SCL-only workflows: structured contract workflows.
- PromptControl + SCL workflows: high-fidelity contract workflows with deterministic caller-side enforcement.

## SCL Parser Module

Package: `scl`  
Primary API:

```go
contract, err := scl.ParseFile("contracts/IRSEV_CONTRACT.md")
if err != nil {
    // handle parse/validation error
}

items := contract.Sections["ISSUE"]
constants := contract.Constants
```

Returned type:

```go
type Contract struct {
    Sections  map[string][]string
    Constants map[string]string

    OrderedConstants []ConstantEntry
    OrderedSections  []SectionEntry
}
```

Render view (deterministic order for output):

```go
render := contract.RenderView()
// top-level order: Constants first, then Sections
```

Template generation API:

```go
tpl := contract.GoTemplate()     // generic Go template text
view := contract.TemplateView()  // data to execute that template
```

## Third-Party Usage

Import path:

```go
import "github.com/PromptFunctions/dev-contracts/scl"
```

Install in your app:

```bash
go get github.com/PromptFunctions/dev-contracts/scl
```

Minimal app usage (similar to `scl/print_contract.go`):

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/PromptFunctions/dev-contracts/scl"
)

func main() {
	contract, err := scl.ParseFile("contracts/IRSEV_CONTRACT.md")
	if err != nil {
		log.Fatal(err)
	}

	// Fast lookup usage
	fmt.Println(contract.Sections["ISSUE"])
	fmt.Println(contract.Constants["SCOPE_CORE"])

	// Deterministic render usage (ordered output)
	render := contract.RenderView()
	out, err := json.MarshalIndent(render, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))

	// Contract template generation + execution
	tplText := contract.GoTemplate()
	tpl, err := template.New("contract").Parse(tplText)
	if err != nil {
		log.Fatal(err)
	}

	var b strings.Builder
	if err := tpl.Execute(&b, contract.TemplateView()); err != nil {
		log.Fatal(err)
	}
	fmt.Println(b.String())
}
```

## Supported SCL DSL

### Constants block

```md
<!-- CONSTANTS:START -->
<pre>
KEY = "value"
</pre>
<!-- CONSTANTS:END -->
```

### Section + routed list block

```md
<!-- $${ -->
## SECTION_NAME
<!-- $$[.term_name -->
- item 1
- item 2
<!-- $$] -->
<!-- $$} -->
```

### Nested route blocks (stack-based)

Valid nesting uses stacked route tokens:

```md
<!-- $${ -->
## EXECUTION
<!-- $$[.block -->
    - block item
<!-- $$[.nested_block -->
    - nested item
<!-- $$] -->
<!-- $$] -->
<!-- $$} -->
```

Not valid:

```md
<!-- $$[.block.nested_block -->
```

Use stacked blocks instead of dotted route paths in one token.

### Constant references resolved by parser

- `<!-- CONSTANTS:$(KEY) -->`
- `$$(KEY)`

### Route term naming

- Valid route term: `[A-Za-z_][A-Za-z0-9_]*`
- Valid: `failure_modes`
- Invalid: `failure-modes`
- Invalid: `block.nested_block` (must be stacked as separate nested tokens)

## Validation Rules (Strict)

- Constants block must exist and use `<pre>...</pre>`.
- Constant lines must match `KEY = "value"`.
- Duplicate constant keys are rejected.
- Section sequence must be:
  `<!-- $${ -->` -> `## NAME` -> `<!-- $$[ -->` -> list items -> `<!-- $$] -->` -> `<!-- $$} -->`.
- Inside list blocks, only `- ` entries are valid.
- Duplicate section names are rejected.
- Unclosed/misordered section/list blocks are rejected.
- Undefined constant references are rejected.

## Determinism Guarantees

- No external dependencies in parser implementation.
- No randomized behavior.
- Line-scoped deterministic parser errors.
- Section list item order is preserved exactly.
- Ordered render output is deterministic and follows source declaration order.

## Tests

Current tests are in:
- `scl/structured_contract_test.go`

Run tests:

```bash
go test ./scl -v
```

Print parsed contract struct (ordered render):

```bash
go run scl/print_contract.go
go run scl/print_contract.go contracts/DELIVERY_CONTRACT_EXPANDED.md
```

## Notes

- Parser is dynamic and generic by design: no hardcoded IRSEV struct fields.
- Map fields are kept for lookup compatibility.
- Ordered slices are used for deterministic rendering/output order.

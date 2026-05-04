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

### Section + list block

```md
<!-- $${ -->
## SECTION_NAME
<!-- $$[ -->
- item 1
- item 2
<!-- $$] -->
<!-- $$} -->
```

### Constant references resolved by parser

- `<!-- CONSTANTS:$(KEY) -->`
- `$$(KEY)`

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

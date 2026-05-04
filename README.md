# dev-contracts

Structured contract definitions and parser utilities for deterministic agent workflows.

## What This Repo Contains

- `contracts/`: Markdown contract files (SCL-compatible contract sources).
- `scl/`: Go SCL parser module.
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

## Tests

Current tests are in:
- `scl/structured_contract_test.go`

Run tests:

```bash
go test ./scl -v
```

If module wiring is not initialized in the current checkout, use:

```bash
GO111MODULE=off go test ./scl -v
```

## Notes

- Parser is dynamic and generic by design: no hardcoded IRSEV struct fields.
- Existing `contracts/IRSEV_CONTRACT.md` must follow SCL token rules for full positive parse coverage.

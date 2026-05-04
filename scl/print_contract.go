//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PromptFunctions/dev-contracts/scl"
)

func main() {
	contractPath := defaultContractPath()
	if len(os.Args) > 1 {
		contractPath = os.Args[1]
	}

	contract, err := scl.ParseFile(contractPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse contract: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(contract.RenderView(), "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal contract: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== STRUCTURED_CONTRACT_JSON ===")
	fmt.Println(string(out))
	fmt.Println("=== STRUCTURED_CONTRACT_TEMPLATE ===")
	fmt.Println(contract.GoTemplate())
}

func defaultContractPath() string {
	candidates := []string{
		"contracts/IRSEV_CONTRACT.md",
		"../contracts/IRSEV_CONTRACT.md",
	}
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		if _, err := os.Stat(clean); err == nil {
			return clean
		}
	}
	return filepath.Clean(candidates[0])
}

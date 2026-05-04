package scl

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	constantsStartToken = "<!-- CONSTANTS:START -->"
	constantsEndToken   = "<!-- CONSTANTS:END -->"
	preStartToken       = "<pre>"
	preEndToken         = "</pre>"

	sectionStartToken = "<!-- $${ -->"
	sectionEndToken   = "<!-- $$} -->"
	listStartToken    = "<!-- $$[ -->"
	listEndToken      = "<!-- $$] -->"
)

var (
	constantLinePattern      = regexp.MustCompile(`^([A-Z0-9_]+)\s*=\s*"([^"]*)"\s*$`)
	htmlConstantRefPattern   = regexp.MustCompile(`<!--\s*CONSTANTS:\$\(([A-Z0-9_]+)\)\s*-->`)
	shortConstantRefPattern  = regexp.MustCompile(`\$\$\(([A-Z0-9_]+)\)`)
	unresolvedMarkerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`<!--\s*CONSTANTS:\$\(`),
		regexp.MustCompile(`\$\$\(`),
	}
)

type Contract struct {
	Sections  map[string][]string
	Constants map[string]string
}

func ParseFile(path string) (*Contract, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(contentBytes)

	constants, err := parseConstants(content)
	if err != nil {
		return nil, err
	}

	sections, err := parseSections(content)
	if err != nil {
		return nil, err
	}

	if err := resolveConstants(sections, constants); err != nil {
		return nil, err
	}

	return &Contract{
		Sections:  sections,
		Constants: constants,
	}, nil
}

func parseConstants(content string) (map[string]string, error) {
	start := strings.Index(content, constantsStartToken)
	if start < 0 {
		return nil, fmt.Errorf("constants block start token %q not found", constantsStartToken)
	}

	end := strings.Index(content, constantsEndToken)
	if end < 0 {
		return nil, fmt.Errorf("constants block end token %q not found", constantsEndToken)
	}
	if end <= start {
		return nil, fmt.Errorf("constants block malformed: end token appears before start token")
	}

	block := content[start+len(constantsStartToken) : end]
	preStart := strings.Index(block, preStartToken)
	preEnd := strings.Index(block, preEndToken)
	if preStart < 0 || preEnd < 0 || preEnd <= preStart {
		return nil, fmt.Errorf("constants block must contain %q ... %q envelope", preStartToken, preEndToken)
	}

	preBody := block[preStart+len(preStartToken) : preEnd]
	scanner := bufio.NewScanner(strings.NewReader(preBody))
	constants := make(map[string]string)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := constantLinePattern.FindStringSubmatch(line)
		if matches == nil {
			return nil, fmt.Errorf("invalid constant line %d: %q", lineNo, line)
		}

		key := matches[1]
		value := matches[2]
		if _, exists := constants[key]; exists {
			return nil, fmt.Errorf("duplicate constant key %q", key)
		}
		constants[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan constants block: %w", err)
	}

	return constants, nil
}

func parseSections(content string) (map[string][]string, error) {
	const (
		stateOutside = iota
		stateExpectHeading
		stateExpectListStart
		stateInList
		stateExpectSectionEnd
	)

	state := stateOutside
	sections := make(map[string][]string)
	currentSection := ""
	currentItems := make([]string, 0, 16)

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		switch state {
		case stateOutside:
			if line == sectionStartToken {
				state = stateExpectHeading
			}
		case stateExpectHeading:
			if !strings.HasPrefix(line, "## ") {
				return nil, fmt.Errorf("line %d: expected section heading after %q", lineNo, sectionStartToken)
			}
			sectionName := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if sectionName == "" {
				return nil, fmt.Errorf("line %d: section heading cannot be empty", lineNo)
			}
			if _, exists := sections[sectionName]; exists {
				return nil, fmt.Errorf("line %d: duplicate section %q", lineNo, sectionName)
			}
			currentSection = sectionName
			currentItems = make([]string, 0, 16)
			state = stateExpectListStart
		case stateExpectListStart:
			if line != listStartToken {
				return nil, fmt.Errorf("line %d: expected list start token %q", lineNo, listStartToken)
			}
			state = stateInList
		case stateInList:
			if line == listEndToken {
				state = stateExpectSectionEnd
				continue
			}
			if line == "" || !strings.HasPrefix(line, "- ") {
				return nil, fmt.Errorf("line %d: invalid list entry %q (must start with '- ')", lineNo, line)
			}
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			currentItems = append(currentItems, item)
		case stateExpectSectionEnd:
			if line != sectionEndToken {
				return nil, fmt.Errorf("line %d: expected section end token %q", lineNo, sectionEndToken)
			}
			sections[currentSection] = currentItems
			currentSection = ""
			currentItems = nil
			state = stateOutside
		default:
			return nil, fmt.Errorf("internal parser error: unknown state %d", state)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan sections: %w", err)
	}

	if state != stateOutside {
		return nil, fmt.Errorf("unexpected EOF: unclosed section/list block")
	}

	return sections, nil
}

func resolveConstants(sections map[string][]string, constants map[string]string) error {
	for sectionName, items := range sections {
		for idx, item := range items {
			resolved, err := replaceConstantRefs(item, constants, htmlConstantRefPattern)
			if err != nil {
				return fmt.Errorf("section %q item %d: %w", sectionName, idx, err)
			}

			resolved, err = replaceConstantRefs(resolved, constants, shortConstantRefPattern)
			if err != nil {
				return fmt.Errorf("section %q item %d: %w", sectionName, idx, err)
			}

			for _, pattern := range unresolvedMarkerPatterns {
				if pattern.MatchString(resolved) {
					return fmt.Errorf("section %q item %d: unresolved constant token %q", sectionName, idx, resolved)
				}
			}

			sections[sectionName][idx] = resolved
		}
	}
	return nil
}

func replaceConstantRefs(input string, constants map[string]string, pattern *regexp.Regexp) (string, error) {
	matches := pattern.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	out := input
	for _, match := range matches {
		full := match[0]
		key := match[1]
		value, ok := constants[key]
		if !ok {
			return "", fmt.Errorf("undefined constant key %q", key)
		}
		out = strings.ReplaceAll(out, full, value)
	}

	return out, nil
}

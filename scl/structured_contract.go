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

type ConstantEntry struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type SectionEntry struct {
	Name  string   `json:"Name"`
	Items []string `json:"Items"`
}

type Contract struct {
	Sections  map[string][]string
	Constants map[string]string

	OrderedConstants []ConstantEntry
	OrderedSections  []SectionEntry
}

type RenderContract struct {
	Constants []ConstantEntry `json:"Constants"`
	Sections  []SectionEntry  `json:"Sections"`
}

func ParseFile(path string) (*Contract, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(contentBytes)

	orderedConstants, constantsMap, err := parseConstants(content)
	if err != nil {
		return nil, err
	}

	orderedSections, sectionsMap, err := parseSections(content)
	if err != nil {
		return nil, err
	}

	if err := resolveConstants(orderedSections, constantsMap); err != nil {
		return nil, err
	}
	mirrorSectionsToMap(orderedSections, sectionsMap)

	return &Contract{
		Sections:         sectionsMap,
		Constants:        constantsMap,
		OrderedConstants: orderedConstants,
		OrderedSections:  orderedSections,
	}, nil
}

func (c *Contract) RenderView() RenderContract {
	constants := make([]ConstantEntry, len(c.OrderedConstants))
	copy(constants, c.OrderedConstants)

	sections := make([]SectionEntry, len(c.OrderedSections))
	for i := range c.OrderedSections {
		items := make([]string, len(c.OrderedSections[i].Items))
		copy(items, c.OrderedSections[i].Items)
		sections[i] = SectionEntry{Name: c.OrderedSections[i].Name, Items: items}
	}

	return RenderContract{
		Constants: constants,
		Sections:  sections,
	}
}

func parseConstants(content string) ([]ConstantEntry, map[string]string, error) {
	start := strings.Index(content, constantsStartToken)
	if start < 0 {
		return nil, nil, fmt.Errorf("constants block start token %q not found", constantsStartToken)
	}

	end := strings.Index(content, constantsEndToken)
	if end < 0 {
		return nil, nil, fmt.Errorf("constants block end token %q not found", constantsEndToken)
	}
	if end <= start {
		return nil, nil, fmt.Errorf("constants block malformed: end token appears before start token")
	}

	block := content[start+len(constantsStartToken) : end]
	preStart := strings.Index(block, preStartToken)
	preEnd := strings.Index(block, preEndToken)
	if preStart < 0 || preEnd < 0 || preEnd <= preStart {
		return nil, nil, fmt.Errorf("constants block must contain %q ... %q envelope", preStartToken, preEndToken)
	}

	preBody := block[preStart+len(preStartToken) : preEnd]
	scanner := bufio.NewScanner(strings.NewReader(preBody))
	ordered := make([]ConstantEntry, 0, 16)
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
			return nil, nil, fmt.Errorf("invalid constant line %d: %q", lineNo, line)
		}

		key := matches[1]
		value := matches[2]
		if _, exists := constants[key]; exists {
			return nil, nil, fmt.Errorf("duplicate constant key %q", key)
		}
		constants[key] = value
		ordered = append(ordered, ConstantEntry{Key: key, Value: value})
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan constants block: %w", err)
	}

	return ordered, constants, nil
}

func parseSections(content string) ([]SectionEntry, map[string][]string, error) {
	const (
		stateOutside = iota
		stateExpectHeading
		stateExpectListStart
		stateInList
		stateExpectSectionEnd
	)

	state := stateOutside
	sectionsMap := make(map[string][]string)
	ordered := make([]SectionEntry, 0, 16)

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
				return nil, nil, fmt.Errorf("line %d: expected section heading after %q", lineNo, sectionStartToken)
			}
			sectionName := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if sectionName == "" {
				return nil, nil, fmt.Errorf("line %d: section heading cannot be empty", lineNo)
			}
			if _, exists := sectionsMap[sectionName]; exists {
				return nil, nil, fmt.Errorf("line %d: duplicate section %q", lineNo, sectionName)
			}
			currentSection = sectionName
			currentItems = make([]string, 0, 16)
			state = stateExpectListStart
		case stateExpectListStart:
			if line != listStartToken {
				return nil, nil, fmt.Errorf("line %d: expected list start token %q", lineNo, listStartToken)
			}
			state = stateInList
		case stateInList:
			if line == listEndToken {
				state = stateExpectSectionEnd
				continue
			}
			if line == "" || !strings.HasPrefix(line, "- ") {
				return nil, nil, fmt.Errorf("line %d: invalid list entry %q (must start with '- ')", lineNo, line)
			}
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			currentItems = append(currentItems, item)
		case stateExpectSectionEnd:
			if line != sectionEndToken {
				return nil, nil, fmt.Errorf("line %d: expected section end token %q", lineNo, sectionEndToken)
			}
			copiedItems := make([]string, len(currentItems))
			copy(copiedItems, currentItems)
			sectionsMap[currentSection] = copiedItems
			ordered = append(ordered, SectionEntry{Name: currentSection, Items: copiedItems})
			currentSection = ""
			currentItems = nil
			state = stateOutside
		default:
			return nil, nil, fmt.Errorf("internal parser error: unknown state %d", state)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan sections: %w", err)
	}

	if state != stateOutside {
		return nil, nil, fmt.Errorf("unexpected EOF: unclosed section/list block")
	}

	return ordered, sectionsMap, nil
}

func resolveConstants(sections []SectionEntry, constants map[string]string) error {
	for sectionIndex, section := range sections {
		for itemIndex, item := range section.Items {
			resolved, err := replaceConstantRefs(item, constants, htmlConstantRefPattern)
			if err != nil {
				return fmt.Errorf("section %q item %d: %w", section.Name, itemIndex, err)
			}

			resolved, err = replaceConstantRefs(resolved, constants, shortConstantRefPattern)
			if err != nil {
				return fmt.Errorf("section %q item %d: %w", section.Name, itemIndex, err)
			}

			for _, pattern := range unresolvedMarkerPatterns {
				if pattern.MatchString(resolved) {
					return fmt.Errorf("section %q item %d: unresolved constant token %q", section.Name, itemIndex, resolved)
				}
			}

			sections[sectionIndex].Items[itemIndex] = resolved
		}
	}
	return nil
}

func mirrorSectionsToMap(ordered []SectionEntry, sectionsMap map[string][]string) {
	for _, section := range ordered {
		items := make([]string, len(section.Items))
		copy(items, section.Items)
		sectionsMap[section.Name] = items
	}
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

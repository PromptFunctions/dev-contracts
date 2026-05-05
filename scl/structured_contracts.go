package scl

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	constantsStartToken = "<!-- CONSTANTS:START -->"
	constantsEndToken   = "<!-- CONSTANTS:END -->"
	preStartToken       = "<pre>"
	preEndToken         = "</pre>"

	sectionStartToken = "<!-- $${ -->"
	sectionEndToken   = "<!-- $$} -->"
	listEndToken      = "<!-- $$] -->"
)

const contractTemplate = `{{define "route"}}
<!-- $$[.{{ .Term }} -->
{{- range .Items }}
    - {{ . }}
{{- end }}
{{- range .Children }}
{{ template "route" . }}
{{- end }}
<!-- $$] -->
{{end}}
# {{ .Title }}

<!-- CONSTANTS:START -->
<pre>
{{- range .Constants }}
    {{ .Key }} = "{{ .Value }}"
{{- end }}
</pre>
<!-- CONSTANTS:END -->
{{- range .Sections }}

---
<!-- $${ -->
## {{ .Name }}
{{- range .Routes }}
{{ template "route" . }}
{{- end }}
<!-- $$} -->
{{- end }}
`

var (
	constantLinePattern      = regexp.MustCompile(`^([A-Z0-9_]+)\s*=\s*"([^"]*)"\s*$`)
	htmlConstantRefPattern   = regexp.MustCompile(`<!--\s*CONSTANTS:\$\(([A-Z0-9_]+)\)\s*-->`)
	shortConstantRefPattern  = regexp.MustCompile(`\$\$\(([A-Z0-9_]+)\)`)
	unresolvedMarkerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`<!--\s*CONSTANTS:\$\(`),
		regexp.MustCompile(`\$\$\(`),
	}
	routeTermPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type ConstantEntry struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type RouteNode struct {
	Term     string      `json:"Term"`
	Path     string      `json:"Path"`
	Items    []string    `json:"Items"`
	Children []RouteNode `json:"Children"`
}

type SectionEntry struct {
	Name   string      `json:"Name"`
	Routes []RouteNode `json:"Routes,omitempty"`
}

type Contract struct {
	Title string

	Sections      map[string][]string
	Constants     map[string]string
	SectionRoutes map[string]map[string][]string

	OrderedConstants []ConstantEntry
	OrderedSections  []SectionEntry
}

type RenderContract struct {
	Constants []ConstantEntry `json:"Constants"`
	Sections  []SectionEntry  `json:"Sections"`
}

type TemplateConstant struct {
	Key    string
	Value  string
	Symbol string
}

type TemplateRoute struct {
	Term     string
	Path     string
	Items    []string
	Children []TemplateRoute
	Symbol   string
}

type TemplateSection struct {
	Name   string
	Routes []TemplateRoute
	Symbol string
}

type TemplateContract struct {
	Title     string
	Constants []TemplateConstant
	Sections  []TemplateSection
}

func ParseFile(path string) (*Contract, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(contentBytes)
	title := parseTitle(content)

	orderedConstants, constantsMap, err := parseConstants(content)
	if err != nil {
		return nil, err
	}

	orderedSections, sectionsMap, sectionRoutes, err := parseSections(content)
	if err != nil {
		return nil, err
	}

	if err := resolveConstants(orderedSections, constantsMap); err != nil {
		return nil, err
	}
	mirrorSectionsToMap(orderedSections, sectionsMap)
	sectionRoutes = buildSectionRoutes(orderedSections)

	return &Contract{
		Title:            title,
		Sections:         sectionsMap,
		Constants:        constantsMap,
		SectionRoutes:    sectionRoutes,
		OrderedConstants: orderedConstants,
		OrderedSections:  orderedSections,
	}, nil
}

func (c *Contract) RenderView() RenderContract {
	constants := make([]ConstantEntry, len(c.OrderedConstants))
	copy(constants, c.OrderedConstants)

	sections := make([]SectionEntry, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sections[i] = copySectionEntry(c.OrderedSections[i])
	}

	return RenderContract{
		Constants: constants,
		Sections:  sections,
	}
}

func (c *Contract) TemplateView() TemplateContract {
	constantNames := make([]string, len(c.OrderedConstants))
	for i := range c.OrderedConstants {
		constantNames[i] = c.OrderedConstants[i].Key
	}
	sectionNames := make([]string, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sectionNames[i] = c.OrderedSections[i].Name
	}

	constantSymbols := buildSymbols(constantNames, "Constant")
	sectionSymbols := buildSymbols(sectionNames, "Section")

	constants := make([]TemplateConstant, len(c.OrderedConstants))
	for i := range c.OrderedConstants {
		constants[i] = TemplateConstant{
			Key:    c.OrderedConstants[i].Key,
			Value:  c.OrderedConstants[i].Value,
			Symbol: constantSymbols[i],
		}
	}

	routeSymbolState := make(map[string]int)
	sections := make([]TemplateSection, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sections[i] = TemplateSection{
			Name:   c.OrderedSections[i].Name,
			Routes: toTemplateRoutes(c.OrderedSections[i].Routes, routeSymbolState),
			Symbol: sectionSymbols[i],
		}
	}

	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = "Structured Contract"
	}

	return TemplateContract{
		Title:     title,
		Constants: constants,
		Sections:  sections,
	}
}

func (c *Contract) GoTemplate() string {
	return contractTemplate
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

type routeBuilder struct {
	term      string
	path      string
	items     []string
	children  []*routeBuilder
	seenChild map[string]struct{}
}

type listStart struct {
	ok   bool
	term string
}

func parseSections(content string) ([]SectionEntry, map[string][]string, map[string]map[string][]string, error) {
	const (
		stateOutside = iota
		stateExpectHeading
		stateInSection
	)

	state := stateOutside
	sectionsMap := make(map[string][]string)
	ordered := make([]SectionEntry, 0, 16)

	currentSection := ""
	currentSectionCanonical := ""
	seenAnyList := false
	topRoutes := make([]*routeBuilder, 0, 8)
	seenTopRouteTerms := make(map[string]struct{})
	routeStack := make([]*routeBuilder, 0, 8)

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0

	resetSectionState := func() {
		seenAnyList = false
		topRoutes = make([]*routeBuilder, 0, 8)
		seenTopRouteTerms = make(map[string]struct{})
		routeStack = make([]*routeBuilder, 0, 8)
	}

	resetSectionState()

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
				return nil, nil, nil, fmt.Errorf("line %d: expected section heading after %q", lineNo, sectionStartToken)
			}
			sectionName := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if sectionName == "" {
				return nil, nil, nil, fmt.Errorf("line %d: section heading cannot be empty", lineNo)
			}
			if _, exists := sectionsMap[sectionName]; exists {
				return nil, nil, nil, fmt.Errorf("line %d: duplicate section %q", lineNo, sectionName)
			}
			currentSection = sectionName
			currentSectionCanonical = strings.ToLower(sectionName)
			resetSectionState()
			state = stateInSection

		case stateInSection:
			if line == sectionEndToken {
				if len(routeStack) > 0 {
					return nil, nil, nil, fmt.Errorf("line %d: section %q closed before all list blocks were closed", lineNo, currentSection)
				}
				if !seenAnyList {
					return nil, nil, nil, fmt.Errorf("line %d: section %q must contain at least one list block", lineNo, currentSection)
				}

				routes := buildRouteTree(topRoutes)
				entry := SectionEntry{
					Name:   currentSection,
					Routes: routes,
				}
				ordered = append(ordered, entry)

				if len(routes) > 0 {
					sectionsMap[currentSection] = copyStringSlice(routes[0].Items)
				} else {
					sectionsMap[currentSection] = []string{}
				}

				currentSection = ""
				currentSectionCanonical = ""
				state = stateOutside
				continue
			}

			start, err := parseListStartToken(line)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			if start.ok {
				seenAnyList = true
				parentPath := currentSectionCanonical
				if len(routeStack) > 0 {
					parentPath = routeStack[len(routeStack)-1].path
				}
				path := parentPath + "." + start.term

				node := &routeBuilder{term: start.term, path: path, items: make([]string, 0, 16), children: make([]*routeBuilder, 0, 4), seenChild: make(map[string]struct{})}

				if len(routeStack) == 0 {
					if _, exists := seenTopRouteTerms[start.term]; exists {
						return nil, nil, nil, fmt.Errorf("line %d: duplicate top-level route %q in section %q", lineNo, start.term, currentSection)
					}
					seenTopRouteTerms[start.term] = struct{}{}
					topRoutes = append(topRoutes, node)
				} else {
					parent := routeStack[len(routeStack)-1]
					if _, exists := parent.seenChild[start.term]; exists {
						return nil, nil, nil, fmt.Errorf("line %d: duplicate nested route %q under path %q", lineNo, start.term, parent.path)
					}
					parent.seenChild[start.term] = struct{}{}
					parent.children = append(parent.children, node)
				}

				routeStack = append(routeStack, node)
				continue
			}

			if line == listEndToken {
				if len(routeStack) == 0 {
					return nil, nil, nil, fmt.Errorf("line %d: unexpected list end token %q", lineNo, listEndToken)
				}

				routeStack = routeStack[:len(routeStack)-1]
				continue
			}

			if strings.HasPrefix(line, "- ") {
				if len(routeStack) == 0 {
					return nil, nil, nil, fmt.Errorf("line %d: list item outside list block", lineNo)
				}
				item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
				routeStack[len(routeStack)-1].items = append(routeStack[len(routeStack)-1].items, item)
				continue
			}

			if line == "" {
				return nil, nil, nil, fmt.Errorf("line %d: blank line inside section %q is not allowed", lineNo, currentSection)
			}

			return nil, nil, nil, fmt.Errorf("line %d: invalid token %q inside section %q", lineNo, line, currentSection)

		default:
			return nil, nil, nil, fmt.Errorf("internal parser error: unknown state %d", state)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("scan sections: %w", err)
	}

	switch state {
	case stateOutside:
		// no-op
	case stateExpectHeading:
		return nil, nil, nil, fmt.Errorf("unexpected EOF: expected section heading")
	case stateInSection:
		return nil, nil, nil, fmt.Errorf("unexpected EOF: section %q not closed", currentSection)
	default:
		return nil, nil, nil, fmt.Errorf("internal parser error: unknown final state %d", state)
	}

	sectionRoutes := buildSectionRoutes(ordered)
	return ordered, sectionsMap, sectionRoutes, nil
}

func parseListStartToken(line string) (listStart, error) {
	if line == "<!-- $$[ -->" {
		return listStart{}, fmt.Errorf("legacy list token %q is not supported; use routed token format <!-- $$[.term -->", "<!-- $$[ -->")
	}

	if !strings.HasPrefix(line, "<!-- $$[.") {
		return listStart{ok: false}, nil
	}
	if !strings.HasSuffix(line, "-->") {
		return listStart{}, fmt.Errorf("invalid routed list start token %q", line)
	}

	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "<!-- $$[."), "-->"))
	if inner == "" {
		return listStart{}, fmt.Errorf("route term cannot be empty")
	}
	if strings.Contains(inner, ".") {
		return listStart{}, fmt.Errorf("invalid route token %q: only single-segment route terms are allowed; use stacked blocks for nesting", line)
	}
	if strings.Contains(inner, "-") {
		return listStart{}, fmt.Errorf("invalid route term %q: hyphen '-' is not allowed, use underscores", inner)
	}
	if !routeTermPattern.MatchString(inner) {
		return listStart{}, fmt.Errorf("invalid route term %q: must match [A-Za-z_][A-Za-z0-9_]*", inner)
	}

	return listStart{ok: true, term: inner}, nil
}

func buildRouteTree(builders []*routeBuilder) []RouteNode {
	out := make([]RouteNode, len(builders))
	for i := range builders {
		out[i] = RouteNode{
			Term:     builders[i].term,
			Path:     builders[i].path,
			Items:    copyStringSlice(builders[i].items),
			Children: buildRouteTree(builders[i].children),
		}
	}
	return out
}

func buildSectionRoutes(sections []SectionEntry) map[string]map[string][]string {
	out := make(map[string]map[string][]string, len(sections))
	for _, section := range sections {
		sectionKey := strings.ToLower(section.Name)
		if _, ok := out[sectionKey]; !ok {
			out[sectionKey] = make(map[string][]string)
		}
		flattenRoutes(out[sectionKey], section.Routes)
	}
	return out
}

func flattenRoutes(out map[string][]string, routes []RouteNode) {
	for _, route := range routes {
		out[route.Path] = copyStringSlice(route.Items)
		flattenRoutes(out, route.Children)
	}
}

func resolveConstants(sections []SectionEntry, constants map[string]string) error {
	for sectionIndex := range sections {
		if err := resolveConstantsOnRoutes(sections[sectionIndex].Name, sections[sectionIndex].Routes, constants); err != nil {
			return err
		}
	}
	return nil
}

func resolveConstantsOnRoutes(sectionName string, routes []RouteNode, constants map[string]string) error {
	for routeIndex := range routes {
		for itemIndex, item := range routes[routeIndex].Items {
			resolved, err := replaceConstantRefs(item, constants, htmlConstantRefPattern)
			if err != nil {
				return fmt.Errorf("section %q route %q item %d: %w", sectionName, routes[routeIndex].Path, itemIndex, err)
			}
			resolved, err = replaceConstantRefs(resolved, constants, shortConstantRefPattern)
			if err != nil {
				return fmt.Errorf("section %q route %q item %d: %w", sectionName, routes[routeIndex].Path, itemIndex, err)
			}
			for _, pattern := range unresolvedMarkerPatterns {
				if pattern.MatchString(resolved) {
					return fmt.Errorf("section %q route %q item %d: unresolved constant token %q", sectionName, routes[routeIndex].Path, itemIndex, resolved)
				}
			}
			routes[routeIndex].Items[itemIndex] = resolved
		}
		if err := resolveConstantsOnRoutes(sectionName, routes[routeIndex].Children, constants); err != nil {
			return err
		}
	}
	return nil
}

func mirrorSectionsToMap(ordered []SectionEntry, sectionsMap map[string][]string) {
	for _, section := range ordered {
		if len(section.Routes) > 0 {
			sectionsMap[section.Name] = copyStringSlice(section.Routes[0].Items)
			continue
		}
		sectionsMap[section.Name] = []string{}
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

func parseTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func buildSymbols(inputs []string, fallbackPrefix string) []string {
	used := make(map[string]int, len(inputs))
	out := make([]string, len(inputs))
	for i, in := range inputs {
		base := normalizeSymbol(in, fallbackPrefix)
		count := used[base]
		if count == 0 {
			out[i] = base
			used[base] = 1
			continue
		}
		count++
		used[base] = count
		out[i] = base + strconv.Itoa(count)
	}
	return out
}

func normalizeSymbol(input, fallbackPrefix string) string {
	var builder strings.Builder
	capitalize := true
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if capitalize {
				builder.WriteRune(unicode.ToUpper(r))
				capitalize = false
			} else {
				builder.WriteRune(unicode.ToLower(r))
			}
			continue
		}
		capitalize = true
	}

	out := builder.String()
	if out == "" {
		out = fallbackPrefix
	}
	if len(out) > 0 {
		first := rune(out[0])
		if unicode.IsDigit(first) {
			out = fallbackPrefix + out
		}
	}
	return out
}

func copyStringSlice(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func copyRouteNodes(in []RouteNode) []RouteNode {
	out := make([]RouteNode, len(in))
	for i := range in {
		out[i] = RouteNode{
			Term:     in[i].Term,
			Path:     in[i].Path,
			Items:    copyStringSlice(in[i].Items),
			Children: copyRouteNodes(in[i].Children),
		}
	}
	return out
}

func copySectionEntry(in SectionEntry) SectionEntry {
	return SectionEntry{
		Name:   in.Name,
		Routes: copyRouteNodes(in.Routes),
	}
}

func toTemplateRoutes(in []RouteNode, symbolState map[string]int) []TemplateRoute {
	out := make([]TemplateRoute, len(in))
	for i := range in {
		symbol := normalizeSymbol(in[i].Term, "Route")
		count := symbolState[symbol]
		if count == 0 {
			symbolState[symbol] = 1
		} else {
			count++
			symbolState[symbol] = count
			symbol = symbol + strconv.Itoa(count)
		}

		out[i] = TemplateRoute{
			Term:     in[i].Term,
			Path:     in[i].Path,
			Items:    copyStringSlice(in[i].Items),
			Children: toTemplateRoutes(in[i].Children, symbolState),
			Symbol:   symbol,
		}
	}
	return out
}

package theory

import (
	"bufio"
	"regexp"
	"strings"
)

var (
	// Session declaration: session "Name" = "Parent" + or session Name = Parent +
	sessionDeclRegex = regexp.MustCompile(`(?i)^\s*session\s+([A-Za-z0-9_\-]+)(?:\s*=\s*([A-Za-z0-9_\-]+|\"[^\"]+\"))?`)
	// Theory header: theory Name imports Imp1 Imp2 ... begin
	theoryStartRegex = regexp.MustCompile(`(?i)^\s*theory\s+([A-Za-z0-9_\-]+)`)
	// Item starters
	itemStartRegex = regexp.MustCompile(`(?i)^\s*(datatype|record|type_synonym|definition|fun|function|inductive|inductive_set|locale|class|instantiation|lemma|theorem|corollary|proposition)\b\s*([A-Za-z0-9_\'\.]*)\s*(.*)`)
	// Sectioning
	sectionRegex = regexp.MustCompile(`(?i)^\s*(chapter|section|subsection|subsubsection)\b\s*(.*)`)
)

// ParseRoot parses Isabelle ROOT file content into structured sessions and dependencies.
func ParseRoot(entryShortname, rawContent string) (*RootFile, error) {
	root := &RootFile{
		EntryShortname: entryShortname,
		RawContent:     rawContent,
	}

	scanner := bufio.NewScanner(strings.NewReader(rawContent))
	var currentSession *Session
	var currentSection string // "sessions", "theories", "directories"

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "(*") || strings.HasPrefix(line, "#") {
			continue
		}

		// Chapter declaration: chapter AFP
		if strings.HasPrefix(strings.ToLower(line), "chapter ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				root.Chapter = parts[1]
			}
			continue
		}

		// New session declaration
		if matches := sessionDeclRegex.FindStringSubmatch(line); len(matches) > 1 {
			if currentSession != nil {
				root.Sessions = append(root.Sessions, *currentSession)
			}
			currentSession = &Session{
				Name: matches[1],
			}
			if len(matches) > 2 {
				parent := strings.Trim(matches[2], `" `)
				currentSession.Parent = parent
			}
			currentSection = ""
			continue
		}

		if currentSession == nil {
			continue
		}

		// Check section headers
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "sessions"):
			currentSection = "sessions"
			parts := strings.Fields(line)[1:]
			for _, p := range parts {
				dep := strings.Trim(p, `" `)
				if dep != "" && dep != "+" {
					currentSession.Sessions = append(currentSession.Sessions, dep)
				}
			}
			continue
		case strings.HasPrefix(lower, "theories"):
			currentSection = "theories"
			parts := strings.Fields(line)[1:]
			for _, p := range parts {
				th := strings.Trim(p, `" `)
				if th != "" && !strings.HasPrefix(th, "[") {
					currentSession.Theories = append(currentSession.Theories, th)
				}
			}
			continue
		case strings.HasPrefix(lower, "directories"):
			currentSection = "directories"
			parts := strings.Fields(line)[1:]
			for _, p := range parts {
				dir := strings.Trim(p, `" `)
				if dir != "" {
					currentSession.Directories = append(currentSession.Directories, dir)
				}
			}
			continue
		case strings.HasPrefix(lower, "document_files") || strings.HasPrefix(lower, "options"):
			currentSection = "other"
			continue
		}

		// Continued list items under current section
		if currentSection == "sessions" {
			for _, p := range strings.Fields(line) {
				dep := strings.Trim(p, `" `)
				if dep != "" && dep != "+" {
					currentSession.Sessions = append(currentSession.Sessions, dep)
				}
			}
		} else if currentSection == "theories" {
			for _, p := range strings.Fields(line) {
				th := strings.Trim(p, `" `)
				if th != "" && !strings.HasPrefix(th, "[") && !strings.HasSuffix(th, "]") {
					currentSession.Theories = append(currentSession.Theories, th)
				}
			}
		}
	}

	if currentSession != nil {
		root.Sessions = append(root.Sessions, *currentSession)
	}

	return root, nil
}

// ParseTheoryOutline extracts the mathematical interface and statements from an Isabelle theory file,
// omitting the internal proof bodies for maximum LLM token efficiency.
func ParseTheoryOutline(entryShortname string, rawContent string) (*TheoryOutline, error) {
	scanner := bufio.NewScanner(strings.NewReader(rawContent))

	outline := &TheoryOutline{
		EntryShortname: entryShortname,
	}

	var inComment bool
	var inProof bool
	var inTheoryHeader bool
	var currentItem *TheoryItem
	var itemBuffer strings.Builder
	var lineNum int

	flushItem := func() {
		if currentItem != nil {
			stmt := strings.TrimSpace(itemBuffer.String())
			currentItem.Statement = stmt
			outline.Items = append(outline.Items, *currentItem)
			currentItem = nil
			itemBuffer.Reset()
		}
	}

	for scanner.Scan() {
		lineNum++
		rawLine := scanner.Text()
		trimmed := strings.TrimSpace(rawLine)
		lower := strings.ToLower(trimmed)

		// Multi-line comment tracking
		if strings.Contains(trimmed, "(*") && !strings.Contains(trimmed, "*)") {
			inComment = true
			continue
		}
		if inComment {
			if strings.Contains(trimmed, "*)") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "(*") && strings.HasSuffix(trimmed, "*)") {
			continue
		}

		// Theory header: theory Foo imports Bar Baz ... begin
		if inTheoryHeader {
			if lower == "begin" || strings.HasSuffix(lower, " begin") {
				inTheoryHeader = false
				cleanLine := strings.TrimSuffix(trimmed, "begin")
				cleanLine = strings.TrimSuffix(cleanLine, "BEGIN")
				for _, imp := range strings.Fields(cleanLine) {
					cleanImp := strings.Trim(imp, `" `)
					if cleanImp != "" && !strings.EqualFold(cleanImp, "imports") {
						outline.Imports = append(outline.Imports, cleanImp)
					}
				}
				outline.Items = append(outline.Items, TheoryItem{
					Kind:      KindTheory,
					Name:      outline.TheoryName,
					Statement: "theory " + outline.TheoryName + " imports " + strings.Join(outline.Imports, " ") + " begin",
					Line:      lineNum,
				})
				continue
			}
			for _, imp := range strings.Fields(trimmed) {
				cleanImp := strings.Trim(imp, `" `)
				if cleanImp != "" && !strings.EqualFold(cleanImp, "imports") {
					outline.Imports = append(outline.Imports, cleanImp)
				}
			}
			continue
		}

		if matches := theoryStartRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			outline.TheoryName = matches[1]
			inTheoryHeader = true
			// Check if "begin" is on the same line
			if strings.Contains(strings.ToLower(trimmed), "begin") {
				inTheoryHeader = false
				// Extract imports between "imports" and "begin"
				impIdx := strings.Index(strings.ToLower(trimmed), "imports")
				beginIdx := strings.Index(strings.ToLower(trimmed), "begin")
				if impIdx != -1 && beginIdx > impIdx {
					rawImports := trimmed[impIdx+len("imports") : beginIdx]
					for _, imp := range strings.Fields(rawImports) {
						cleanImp := strings.Trim(imp, `" `)
						if cleanImp != "" {
							outline.Imports = append(outline.Imports, cleanImp)
						}
					}
				}
				outline.Items = append(outline.Items, TheoryItem{
					Kind:      KindTheory,
					Name:      outline.TheoryName,
					Statement: trimmed,
					Line:      lineNum,
				})
			} else {
				// Imports might start on this line after "imports"
				impIdx := strings.Index(strings.ToLower(trimmed), "imports")
				if impIdx != -1 {
					rawImports := trimmed[impIdx+len("imports"):]
					for _, imp := range strings.Fields(rawImports) {
						cleanImp := strings.Trim(imp, `" `)
						if cleanImp != "" {
							outline.Imports = append(outline.Imports, cleanImp)
						}
					}
				}
			}
			continue
		}

		// Check proof state transitions
		if lower == "proof" || strings.HasPrefix(lower, "proof ") ||
			lower == "by " || strings.HasPrefix(lower, "by ") ||
			strings.HasPrefix(lower, "apply ") || lower == "sorry" || lower == "oops" {
			flushItem()
			inProof = true
		}
		if inProof {
			if lower == "qed" || lower == "done" || lower == "sorry" || lower == "oops" || strings.HasPrefix(lower, "by ") {
				inProof = false
			}
			continue
		}

		// Section headers
		if secMatches := sectionRegex.FindStringSubmatch(trimmed); len(secMatches) > 1 {
			flushItem()
			outline.Items = append(outline.Items, TheoryItem{
				Kind:      KindSection,
				Name:      secMatches[1],
				Statement: trimmed,
				Line:      lineNum,
			})
			continue
		}

		// Declaration / Lemma / Theorem starter
		if matches := itemStartRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
			flushItem()

			rawKind := strings.ToLower(matches[1])
			name := matches[2]
			var kind ItemKind

			switch rawKind {
			case "datatype", "record", "type_synonym":
				kind = KindType
				outline.TotalTypes++
			case "definition", "instantiation":
				kind = KindDefinition
				outline.TotalDefs++
			case "fun", "function", "inductive", "inductive_set":
				kind = KindFunction
				outline.TotalDefs++
			case "locale":
				kind = KindLocale
			case "class":
				kind = KindClass
			case "lemma":
				kind = KindLemma
				outline.TotalLemmas++
			case "theorem", "corollary", "proposition":
				kind = KindTheorem
				outline.TotalTheorems++
			default:
				kind = KindDefinition
			}

			currentItem = &TheoryItem{
				Kind: kind,
				Name: name,
				Line: lineNum,
			}
			itemBuffer.WriteString(trimmed)

			// If statement finishes on the same line (no multi-line body)
			if strings.HasSuffix(trimmed, "begin") || strings.HasSuffix(trimmed, `"`) || !strings.Contains(trimmed, `"`) {
				// Continue collecting if needed or check next line
			}
			continue
		}

		// Collecting multi-line statement (e.g. multi-line lemma or datatype definition)
		if currentItem != nil {
			if itemBuffer.Len() < 500 { // Bound size for token efficiency
				itemBuffer.WriteString("\n  ")
				itemBuffer.WriteString(trimmed)
			}
			// End of definition/datatype or start of proof
			if strings.HasSuffix(trimmed, `"`) || strings.HasSuffix(trimmed, "begin") || strings.Contains(trimmed, "proof") {
				flushItem()
			}
		}
	}

	flushItem()

	outline.OriginalLines = lineNum
	outline.OutlineLines = len(outline.Items)

	return outline, nil
}

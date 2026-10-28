package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
)

// FormatSearchResult formats search matches based on requested format ("compact", "detailed", "json").
func FormatSearchResult(res *search.Result, format string) (string, error) {
	if format == "json" {
		bytes, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	if len(res.Matches) == 0 {
		return "No matching AFP entries found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d entries (showing %d–%d, took %dµs):\n\n",
		res.TotalHits, res.Offset+1, res.Offset+len(res.Matches), res.TookMicrosecond))

	for i, m := range res.Matches {
		e := m.Entry
		if format == "detailed" {
			sb.WriteString(fmt.Sprintf("### %s (%s)\n", e.Shortname, e.Year))
			sb.WriteString(fmt.Sprintf("**Title:** %s\n", e.Title))
			sb.WriteString(fmt.Sprintf("**Authors:** %s\n", strings.Join(e.Authors, ", ")))
			sb.WriteString(fmt.Sprintf("**Topics:** %s\n", strings.Join(e.Topics, ", ")))
			sb.WriteString(fmt.Sprintf("**Tarball:** %s\n", e.ReleaseTarballURL()))
			sb.WriteString(fmt.Sprintf("**Permalink:** %s\n", e.Permalink))
			if m.Snippet != "" {
				sb.WriteString(fmt.Sprintf("**Snippet:** %s\n", m.Snippet))
			}
		} else { // "compact" (default for agents)
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", e.Shortname, e.Year, e.Title))
			sb.WriteString(fmt.Sprintf("  Topics: %s | Score: %.2f\n", strings.Join(e.Topics, ", "), m.Score))
			if m.Snippet != "" {
				sb.WriteString(fmt.Sprintf("  Abstract: %s\n", m.Snippet))
			}
		}

		if i < len(res.Matches)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// FormatEntry formats an aggregate entry into clean markdown.
func FormatEntry(e *entry.Entry) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s: %s\n\n", e.Shortname, e.Title))
	sb.WriteString(fmt.Sprintf("- **Release Date:** %s (%s)\n", e.Date, e.Year))
	sb.WriteString(fmt.Sprintf("- **Authors:** %s\n", strings.Join(e.Authors, ", ")))
	sb.WriteString(fmt.Sprintf("- **Topics:** %s\n", strings.Join(e.Topics, ", ")))
	sb.WriteString(fmt.Sprintf("- **Package Download:** %s\n", e.ReleaseTarballURL()))
	sb.WriteString(fmt.Sprintf("- **AFP Web Page:** %s\n\n", e.Permalink))

	sb.WriteString("## Abstract\n")
	sb.WriteString(e.Abstract)
	sb.WriteString("\n\n")

	if len(e.Related) > 0 {
		sb.WriteString("## Related Publications / References\n")
		for _, r := range e.Related {
			if r.DOI != "" {
				sb.WriteString(fmt.Sprintf("- %s (DOI: %s)\n", r.HTML, r.DOI))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", r.HTML))
			}
		}
	}

	return sb.String()
}

// FormatTheoryOutline formats the extracted interface of a .thy file.
func FormatTheoryOutline(outline *theory.TheoryOutline) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Theory Outline: %s (Entry: %s)\n\n", outline.TheoryName, outline.EntryShortname))
	sb.WriteString(fmt.Sprintf("- **Imports:** %s\n", strings.Join(outline.Imports, ", ")))
	sb.WriteString(fmt.Sprintf("- **Statistics:** %d types, %d defs/funs, %d lemmas, %d theorems (reduced from %d lines to %d interface items)\n\n",
		outline.TotalTypes, outline.TotalDefs, outline.TotalLemmas, outline.TotalTheorems, outline.OriginalLines, outline.OutlineLines))

	sb.WriteString("## Mathematical Declarations & Statements\n\n```isabelle\n")
	for _, item := range outline.Items {
		sb.WriteString(fmt.Sprintf("(* Line %d: %s *)\n", item.Line, item.Kind))
		sb.WriteString(item.Statement)
		sb.WriteString("\n\n")
	}
	sb.WriteString("```\n")

	return sb.String()
}

// FormatRoot formats parsed ROOT file session dependencies.
func FormatRoot(root *theory.RootFile) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Isabelle ROOT for AFP Entry: %s\n\n", root.EntryShortname))
	if root.Chapter != "" {
		sb.WriteString(fmt.Sprintf("**Chapter:** %s\n\n", root.Chapter))
	}

	allDeps := root.AllDependencies()
	if len(allDeps) > 0 {
		sb.WriteString(fmt.Sprintf("**All Session Dependencies:** %s\n\n", strings.Join(allDeps, ", ")))
	}

	for _, s := range root.Sessions {
		sb.WriteString(fmt.Sprintf("## Session `%s`\n", s.Name))
		if s.Parent != "" {
			sb.WriteString(fmt.Sprintf("- **Parent Session:** %s\n", s.Parent))
		}
		if len(s.Sessions) > 0 {
			sb.WriteString(fmt.Sprintf("- **AFP/HOL Sessions:** %s\n", strings.Join(s.Sessions, ", ")))
		}
		if len(s.Theories) > 0 {
			sb.WriteString(fmt.Sprintf("- **Theories (%d):**\n", len(s.Theories)))
			for _, th := range s.Theories {
				sb.WriteString(fmt.Sprintf("  - `%s`\n", th))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatTheoriesList formats the file inventory of an entry.
func FormatTheoriesList(entryID string, files []theory.TheoryFileInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Theories & Files in Entry `%s` (%d files)\n\n", entryID, len(files)))

	var roots []theory.TheoryFileInfo
	var thys []theory.TheoryFileInfo

	for _, f := range files {
		if f.IsRoot {
			roots = append(roots, f)
		} else {
			thys = append(thys, f)
		}
	}

	if len(roots) > 0 {
		sb.WriteString("## ROOT Specifications\n")
		for _, r := range roots {
			sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", r.Path, humanSize(r.SizeBytes)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("## Theory Files (.thy) [%d]\n", len(thys)))
	for _, t := range thys {
		sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", t.Path, humanSize(t.SizeBytes)))
	}

	return sb.String()
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

package theory

// TheoryFileInfo describes a theory (.thy) file within an AFP entry package.
type TheoryFileInfo struct {
	EntryShortname string `json:"entry_shortname"`
	Path           string `json:"path"` // Relative path inside tarball (e.g. "Formal_SSA/Construct_SSA.thy")
	Name           string `json:"name"` // Theory base name (e.g. "Construct_SSA")
	SizeBytes      int64  `json:"size_bytes"`
	IsRoot         bool   `json:"is_root,omitempty"`
}

// TheoryContent holds the text content (full or sliced) of a theory or root file.
type TheoryContent struct {
	EntryShortname string `json:"entry_shortname"`
	Path           string `json:"path"`
	TotalLines     int    `json:"total_lines"`
	LineStart      int    `json:"line_start"`
	LineEnd        int    `json:"line_end"`
	Content        string `json:"content"`
	Truncated      bool   `json:"truncated"`
}

// ItemKind categorizes Isabelle mathematical statements.
type ItemKind string

const (
	KindHeader     ItemKind = "header"
	KindTheory     ItemKind = "theory"
	KindType       ItemKind = "type"
	KindDefinition ItemKind = "definition"
	KindFunction   ItemKind = "function"
	KindLocale     ItemKind = "locale"
	KindClass      ItemKind = "class"
	KindTheorem    ItemKind = "theorem"
	KindLemma      ItemKind = "lemma"
	KindSection    ItemKind = "section"
)

// TheoryItem represents an extracted specification item (e.g. lemma statement, type definition).
type TheoryItem struct {
	Kind      ItemKind `json:"kind"`
	Name      string   `json:"name,omitempty"`
	Statement string   `json:"statement"`
	Line      int      `json:"line"`
}

// TheoryOutline is an agent-optimized representation of a .thy file containing the mathematical
// interface (imports, types, definitions, theorem statements) without bulky proof scripts.
type TheoryOutline struct {
	EntryShortname string       `json:"entry_shortname"`
	TheoryName     string       `json:"theory_name"`
	Imports        []string     `json:"imports,omitempty"`
	Items          []TheoryItem `json:"items"`
	TotalLemmas    int          `json:"total_lemmas"`
	TotalTheorems  int          `json:"total_theorems"`
	TotalDefs      int          `json:"total_defs"`
	TotalTypes     int          `json:"total_types"`
	OriginalLines  int          `json:"original_lines"`
	OutlineLines   int          `json:"outline_lines"`
}

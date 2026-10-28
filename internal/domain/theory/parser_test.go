package theory

import (
	"testing"
)

func TestParseRoot(t *testing.T) {
	rawRoot := `chapter AFP

session Formal_SSA = Collections +
  options [timeout = 1200]
  sessions
    Dijkstra_Shortest_Path
    Slicing
    CAVA_Automata
  theories
    FormalSSA_Misc
    Serial_Rel
    Construct_SSA
    Construct_SSA_code
  document_files
    "root.tex"
    "root.bib"
`

	root, err := ParseRoot("Formal_SSA", rawRoot)
	if err != nil {
		t.Fatalf("ParseRoot failed: %v", err)
	}

	if root.EntryShortname != "Formal_SSA" {
		t.Errorf("expected entry shortname Formal_SSA, got %s", root.EntryShortname)
	}
	if root.Chapter != "AFP" {
		t.Errorf("expected chapter AFP, got %s", root.Chapter)
	}
	if len(root.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(root.Sessions))
	}

	s := root.Sessions[0]
	if s.Name != "Formal_SSA" {
		t.Errorf("expected session name Formal_SSA, got %s", s.Name)
	}
	if s.Parent != "Collections" {
		t.Errorf("expected parent Collections, got %s", s.Parent)
	}

	deps := s.Sessions
	if len(deps) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(deps))
	}
	if deps[0] != "Dijkstra_Shortest_Path" || deps[1] != "Slicing" || deps[2] != "CAVA_Automata" {
		t.Errorf("unexpected dependencies: %v", deps)
	}

	theories := s.Theories
	if len(theories) != 4 {
		t.Fatalf("expected 4 theories, got %d", len(theories))
	}
	if theories[0] != "FormalSSA_Misc" || theories[2] != "Construct_SSA" {
		t.Errorf("unexpected theories: %v", theories)
	}

	allDeps := root.AllDependencies()
	if len(allDeps) != 4 { // Collections + 3 sessions
		t.Errorf("expected 4 total dependencies, got %d (%v)", len(allDeps), allDeps)
	}
}

func TestParseTheoryOutline(t *testing.T) {
	rawThy := `(*  Title:      Construct_SSA.thy
    Author:     Sebastian Ullrich
*)

section \<open>SSA Construction\<close>
subsection \<open>CFG to SSA CFG\<close>

theory Construct_SSA imports SSA_CFG "HOL-Library.While_Combinator"
begin

datatype Def = SimpleDef | PhiDef
type_synonym ('node, 'var) ssaVal = "'var \<times> 'node \<times> Def"

definition "is_phi x \<longleftrightarrow> x = PhiDef"

fun phi_nodes :: "'g \<Rightarrow> 'var \<Rightarrow> 'node set" where
  "phi_nodes g v = {}"

lemma phi_nodes_empty:
  "phi_nodes g v = {}"
  by simp

theorem ssa_construction_sound:
  assumes "valid_cfg g"
  shows "is_ssa (construct_ssa g)"
proof -
  have 1: "True" by simp
  then show ?thesis
    by blast
qed

end
`

	outline, err := ParseTheoryOutline("Formal_SSA", rawThy)
	if err != nil {
		t.Fatalf("ParseTheoryOutline failed: %v", err)
	}

	if outline.TheoryName != "Construct_SSA" {
		t.Errorf("expected theory name Construct_SSA, got %s", outline.TheoryName)
	}
	if len(outline.Imports) != 2 {
		t.Errorf("expected 2 imports, got %d (%v)", len(outline.Imports), outline.Imports)
	}
	if outline.TotalLemmas != 1 {
		t.Errorf("expected 1 lemma, got %d", outline.TotalLemmas)
	}
	if outline.TotalTheorems != 1 {
		t.Errorf("expected 1 theorem, got %d", outline.TotalTheorems)
	}
	if outline.TotalTypes != 2 { // datatype + type_synonym
		t.Errorf("expected 2 types, got %d", outline.TotalTypes)
	}
	if outline.TotalDefs != 2 { // definition + fun
		t.Errorf("expected 2 defs, got %d", outline.TotalDefs)
	}

	// Verify that proof tactic lines (by simp, have 1: "True", by blast, qed) are NOT in the outline items
	for _, item := range outline.Items {
		if item.Kind == KindTheorem {
			if item.Name != "ssa_construction_sound" {
				t.Errorf("expected theorem name ssa_construction_sound, got %s", item.Name)
			}
			if item.Statement == "" {
				t.Errorf("theorem statement should not be empty")
			}
		}
	}
}

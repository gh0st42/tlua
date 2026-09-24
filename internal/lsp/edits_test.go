package lsp

import "testing"

func TestApplyEditsSingleLine(t *testing.T) {
	text := "local x=1\n"
	edits := []TextEdit{{Range: Range{Position{0, 7}, Position{0, 8}}, NewText: " = "}}

	got, err := ApplyEdits(text, edits)
	if err != nil {
		t.Fatal(err)
	}
	if want := "local x = 1\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Edits all refer to the original document and arrive in no particular order.
func TestApplyEditsOutOfOrder(t *testing.T) {
	text := "a\nb\nc\n"
	edits := []TextEdit{
		{Range: Range{Position{2, 0}, Position{2, 1}}, NewText: "third"},
		{Range: Range{Position{0, 0}, Position{0, 1}}, NewText: "first"},
		{Range: Range{Position{1, 0}, Position{1, 1}}, NewText: "second"},
	}

	got, err := ApplyEdits(text, edits)
	if err != nil {
		t.Fatal(err)
	}
	if want := "first\nsecond\nthird\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyEditsWholeDocument(t *testing.T) {
	text := "old\ncontent\n"
	edits := []TextEdit{{
		Range:   Range{Position{0, 0}, Position{2, 0}},
		NewText: "brand new\n",
	}}

	got, err := ApplyEdits(text, edits)
	if err != nil {
		t.Fatal(err)
	}
	if want := "brand new\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyEditsInsertionsAndDeletions(t *testing.T) {
	// An empty range inserts; an empty newText deletes.
	text := "ac\n"
	got, err := ApplyEdits(text, []TextEdit{
		{Range: Range{Position{0, 1}, Position{0, 1}}, NewText: "b"},
	})
	if err != nil || got != "abc\n" {
		t.Errorf("insert: %q, %v", got, err)
	}

	got, err = ApplyEdits("abc\n", []TextEdit{
		{Range: Range{Position{0, 1}, Position{0, 2}}, NewText: ""},
	})
	if err != nil || got != "ac\n" {
		t.Errorf("delete: %q, %v", got, err)
	}
}

func TestApplyEditsRejectsOverlaps(t *testing.T) {
	text := "hello world\n"
	edits := []TextEdit{
		{Range: Range{Position{0, 0}, Position{0, 7}}, NewText: "x"},
		{Range: Range{Position{0, 5}, Position{0, 11}}, NewText: "y"},
	}
	if _, err := ApplyEdits(text, edits); err == nil {
		t.Error("overlapping edits were accepted")
	}
}

func TestApplyEditsNoEdits(t *testing.T) {
	if got, err := ApplyEdits("unchanged\n", nil); err != nil || got != "unchanged\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

// Positions count UTF-16 code units, so a character outside the basic plane
// counts twice and a byte-based reading of it would cut a rune in half.
func TestOffsetCountsUTF16Units(t *testing.T) {
	text := "-- \U0001F600 x\nsecond\n" // the emoji is two UTF-16 units, four bytes

	cases := map[Position]int{
		{0, 0}:  0,
		{0, 3}:  3,  // before the emoji
		{0, 5}:  7,  // after it: two units, four bytes
		{0, 7}:  9,  // after " x"
		{1, 0}:  10, // the second line
		{1, 6}:  16,
		{2, 0}:  17, // the empty line after the final newline
		{9, 0}:  17, // past the end clamps
		{1, 99}: 16, // past the end of a line clamps to its end
	}
	for pos, want := range cases {
		got, err := Offset(text, pos)
		if err != nil {
			t.Fatalf("%v: %v", pos, err)
		}
		if got != want {
			t.Errorf("Offset(%d:%d) = %d, want %d", pos.Line, pos.Character, got, want)
		}
	}
}

func TestApplyEditsAroundWideCharacters(t *testing.T) {
	text := "local s = \"\U0001F600\"\n"
	// Replace the emoji, addressed in UTF-16 units.
	edits := []TextEdit{{Range: Range{Position{0, 11}, Position{0, 13}}, NewText: "ok"}}

	got, err := ApplyEdits(text, edits)
	if err != nil {
		t.Fatal(err)
	}
	if want := "local s = \"ok\"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestOffsetRejectsNegativePositions(t *testing.T) {
	if _, err := Offset("x\n", Position{-1, 0}); err == nil {
		t.Error("a negative line was accepted")
	}
}

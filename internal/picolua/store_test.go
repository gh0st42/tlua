package picolua

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// saved is a runtime with somewhere to save: a folder held in memory, which is
// what the host gives a real program in the person's own application data.
func saved(t *testing.T, src string) (*fixture, map[string][]byte) {
	t.Helper()
	files := map[string][]byte{}
	f := startWith(t, Options{
		Width: 8, Height: 4, Seed: 1,
		ReadSave: func(name string) ([]byte, error) {
			data, ok := files[name]
			if !ok {
				return nil, errNotThere
			}
			return data, nil
		},
		WriteSave: func(name string, data []byte) error {
			files[name] = data
			return nil
		},
		ReadFile: func(name string) ([]byte, error) { return nil, errNotThere },
	}, src)
	return f, files
}

func TestWhatIsSavedComesBackTheSame(t *testing.T) {
	f, files := saved(t, `
		ok = store("game", {
			level = 3,
			name = "ana",
			won = true,
			ratio = 0.25,
			items = { "rope", "lamp" },
			where = { x = -2, y = 40.5 },
			["high score"] = 1200,
		})
		back = fetch("game")`)

	if got := f.str(`ok`); got != "true" {
		t.Fatalf("store said %q", got)
	}
	if _, ok := files["game.txt"]; !ok {
		t.Fatalf("nothing was written: %v", files)
	}

	cases := []struct{ expr, want string }{
		{`back.level`, "3"},
		{`back.name`, "ana"},
		{`back.won`, "true"},
		{`back.ratio`, "0.25"},
		{`#back.items`, "2"},
		{`back.items[2]`, "lamp"},
		{`back.where.x, back.where.y`, "-2,40.5"},
		{`back["high score"]`, "1200"},
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestASavedFileIsMeantToBeReadByAPerson(t *testing.T) {
	// The point of text: whoever is writing the game will open this file, and
	// may well fix something in it. Saving the same thing twice gives the same
	// bytes, so a save can be kept in version control without churning.
	f, files := saved(t, `
		store("game", { 2, 1, zebra = true, apple = 1, ["a b"] = 2 })
		store("again", { 2, 1, zebra = true, apple = 1, ["a b"] = 2 })`)
	_ = f

	want := `-- tlua store 1
{
	2,
	1,
	["a b"] = 2,
	apple = 1,
	zebra = true,
}
`
	if got := string(files["game.txt"]); got != want {
		t.Errorf("the file reads\n%s\nwant\n%s", got, want)
	}
	if string(files["again.txt"]) != string(files["game.txt"]) {
		t.Error("the same value saved twice gave two different files")
	}
}

func TestSavingSomethingThatCannotBeSaved(t *testing.T) {
	f, _ := saved(t, `
		loop = {}
		loop.self = loop
		_, cycle = store("a", loop)
		_, fn = store("b", { go = print })
		_, path = store("../elsewhere", {})
		_, slash = store("dir/name", {})
		_, empty = store("", {})`)

	for _, c := range []struct{ name, contains string }{
		{"cycle", "holds itself"},
		{"fn", "function cannot be saved"},
		{"path", "not a path"},
		{"slash", "not a path"},
		{"empty", "wants a name"},
	} {
		if got := f.str(c.name); !strings.Contains(got, c.contains) {
			t.Errorf("%s said %q, want something about %q", c.name, got, c.contains)
		}
	}
}

func TestAProgramWithNowhereToSave(t *testing.T) {
	// A runtime with no host behind it — a test, or a program run somewhere
	// with no home directory — can still run; it just cannot save.
	f := start(t, 4, 4, `ok, err = store("game", { 1 })`)
	if got := f.str(`ok == nil`); got != "true" {
		t.Errorf("store said %q", got)
	}
	if got := f.str(`err`); !strings.Contains(got, "nowhere to save") {
		t.Errorf("it said %q", got)
	}
}

func TestASaveIsPreferredToTheFileTheGameShippedWith(t *testing.T) {
	// A game ships its defaults and the player's own version takes over once
	// there is one, which is the whole reason fetch looks at both.
	files := map[string][]byte{}
	f := startWith(t, Options{
		Width: 4, Height: 4,
		ReadSave: func(name string) ([]byte, error) {
			data, ok := files[name]
			if !ok {
				return nil, errNotThere
			}
			return data, nil
		},
		WriteSave: func(name string, data []byte) error { files[name] = data; return nil },
		ReadFile: func(name string) ([]byte, error) {
			if name == "settings.txt" {
				return []byte("shipped"), nil
			}
			return nil, errNotThere
		},
	}, `
		before = fetch("settings")
		store("settings", { sound = false })
		after = fetch("settings")`)

	if got := f.str(`before`); got != "shipped" {
		t.Errorf("before saving, fetch gave %q", got)
	}
	if got := f.str(`type(after), tostr(after.sound)`); got != "table,false" {
		t.Errorf("after saving, fetch gave %q", got)
	}
}

func TestAPlainFileIsStillAString(t *testing.T) {
	// fetch is not only for saved values: a level or a table of numbers the
	// game shipped with comes back as the text it is.
	f := startWith(t, Options{
		Width: 4, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			if name == "level.txt" {
				return []byte("1 2 3"), nil
			}
			return nil, errNotThere
		},
	}, `text = fetch("level")`)

	if got := f.str(`text`); got != "1 2 3" {
		t.Errorf("fetch gave %q", got)
	}
}

// TestReadingASaveThatSomebodyHasEditedBadly is what happens after a person
// opens the file and makes a mistake in it, which is a thing text invites.
func TestReadingASaveThatSomebodyHasEditedBadly(t *testing.T) {
	broken := map[string]string{
		"truncated": "-- tlua store 1\n{ 1, 2,",
		"nonsense":  "-- tlua store 1\n{ 1, @ }",
		"trailing":  "-- tlua store 1\n{ 1 } and then some",
		"unclosed":  "-- tlua store 1\n{ name = \"ana }",
	}
	for name, text := range broken {
		t.Run(name, func(t *testing.T) {
			f := startWith(t, Options{
				Width: 4, Height: 4,
				ReadSave: func(string) ([]byte, error) { return []byte(text), nil },
			}, `value, err = fetch("game")`)

			if got := f.str(`value == nil`); got != "true" {
				t.Errorf("a broken save read as %q", f.str(`tostr(value)`))
			}
			if got := f.str(`err`); !strings.Contains(got, "line") {
				t.Errorf("the complaint was %q, which does not say where", got)
			}
		})
	}
}

// TestASaveIsReadRatherThanRun is the reason for the parser: a save file is a
// file on somebody's disk, and running it would make it a place to put code.
func TestASaveIsReadRatherThanRun(t *testing.T) {
	const attack = "-- tlua store 1\n{ x = (function() ran = true return 1 end)() }"
	f := startWith(t, Options{
		Width: 4, Height: 4,
		ReadSave: func(string) ([]byte, error) { return []byte(attack), nil },
	}, `value, err = fetch("game")`)

	if got := f.str(`ran == nil`); got != "true" {
		t.Error("the save file ran")
	}
	if got := f.str(`value == nil`); got != "true" {
		t.Errorf("it read as %q", f.str(`tostr(value)`))
	}
}

func TestTheShapesAValueCanTake(t *testing.T) {
	// The encoder and the parser, without a runtime in the way: every kind of
	// value that can be saved, written out and read back.
	L := lua.NewState()
	defer L.Close()

	for _, src := range []string{
		`{}`,
		`{ 1, 2, 3 }`,
		`{ a = 1, b = "two", c = true, d = false }`,
		`{ { { { 1 } } } }`,
		`{ ["and"] = 1, ["end"] = 2, ["1"] = 3, [1.5] = 4 }`,
		`{ big = 1e15, small = 0.000001, neg = -17 }`,
		`{ text = "tabs\tand \"quotes\" and \\ slashes" }`,

		// A value on its own is as savable as a table of them: a high score is
		// a number, and asking a program to wrap it in a table to keep it
		// would be a rule with nothing behind it.
		`12`,
		`-0.5`,
		`"a string"`,
		`true`,
	} {
		if err := L.DoString(`value = ` + src); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		text, err := encode(L.GetGlobal("value"))
		if err != nil {
			t.Errorf("%s: encoding: %v", src, err)
			continue
		}
		back, err := decode(L, text)
		if err != nil {
			t.Errorf("%s: reading back %s: %v", src, text, err)
			continue
		}
		// The proof is that saving what came back gives the same file again.
		again, err := encode(back)
		if err != nil {
			t.Errorf("%s: encoding what came back: %v", src, err)
			continue
		}
		if again != text {
			t.Errorf("%s does not survive the trip:\n%s\nbecame\n%s", src, text, again)
		}
	}
}

package picolua

import (
	"math"
	"math/rand"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installStdlib adds the short helpers that PICO-8 and Picotron programs are
// written with. They are all available in Lua's own libraries in one form or
// another; having them under these names, with these edges, is what lets a
// program from that world run here and read the way it was written.
func (r *Runtime) installStdlib() {
	r.register(map[string]lua.LGFunction{
		"flr":  num1(math.Floor),
		"ceil": num1(math.Ceil),
		"abs":  num1(math.Abs),

		// sqrt of a negative number is 0 rather than an error, so that a
		// distance calculation that drifts below zero does not stop the game.
		"sqrt": num1(func(v float64) float64 {
			if v <= 0 {
				return 0
			}
			return math.Sqrt(v)
		}),

		// sgn(0) is 1, as it is on PICO-8: the sign of something that is not
		// moving has to be one or the other, and this is the one that was
		// chosen there.
		"sgn": num1(func(v float64) float64 {
			if v < 0 {
				return -1
			}
			return 1
		}),

		// Angles are turns, not radians: a whole circle is 1. sin() runs the
		// same way round as the screen's y axis, which is downwards, so that
		// sin and cos of the same angle move a sprite the way the numbers look.
		"sin": num1(func(v float64) float64 { return -math.Sin(v * 2 * math.Pi) }),
		"cos": num1(func(v float64) float64 { return math.Cos(v * 2 * math.Pi) }),

		// atan2(dx, dy) is the turn that sin and cos would give those numbers
		// back for, between 0 and 1.
		"atan2": func(L *lua.LState) int {
			dx, dy := float64(L.CheckNumber(1)), float64(L.CheckNumber(2))
			turn := math.Atan2(-dy, dx) / (2 * math.Pi)
			L.Push(lua.LNumber(turn - math.Floor(turn)))
			return 1
		},

		"min": func(L *lua.LState) int {
			L.Push(lua.LNumber(math.Min(float64(L.CheckNumber(1)), float64(L.OptNumber(2, 0)))))
			return 1
		},
		"max": func(L *lua.LState) int {
			L.Push(lua.LNumber(math.Max(float64(L.CheckNumber(1)), float64(L.OptNumber(2, 0)))))
			return 1
		},

		// mid(a, b, c) is the middle of the three, which is how these programs
		// keep a value inside a range.
		"mid": func(L *lua.LState) int {
			a, b, c := float64(L.CheckNumber(1)), float64(L.CheckNumber(2)), float64(L.CheckNumber(3))
			L.Push(lua.LNumber(math.Max(math.Min(a, b), math.Min(math.Max(a, b), c))))
			return 1
		},

		// clamp(x, lo, hi) keeps a number between two others. It is mid() by
		// another name, and the name is the point: "clamp" says what it is for
		// where "the middle of three" says what it does.
		"clamp": func(L *lua.LState) int {
			x := float64(L.CheckNumber(1))
			lo, hi := float64(L.CheckNumber(2)), float64(L.CheckNumber(3))
			if lo > hi {
				lo, hi = hi, lo
			}
			L.Push(lua.LNumber(math.Max(lo, math.Min(x, hi))))
			return 1
		},

		// rnd() is a fraction below one, rnd(n) a number below n, and rnd(table)
		// one of the things in it.
		"rnd": func(L *lua.LState) int {
			switch v := L.Get(1).(type) {
			case *lua.LTable:
				n := v.Len()
				if n == 0 {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(v.RawGetInt(r.rng.Intn(n) + 1))
				return 1
			case lua.LNumber:
				L.Push(lua.LNumber(r.rng.Float64() * float64(v)))
				return 1
			default:
				L.Push(lua.LNumber(r.rng.Float64()))
				return 1
			}
		},

		// srand(seed) makes the sequence repeat, for a level that should be the
		// same every time it is generated.
		"srand": func(L *lua.LState) int {
			r.reseed(int64(L.OptNumber(1, 0)))
			return 0
		},

		// add(t, v) puts v at the end and reports it back, so that the thing
		// just added can be used in the same breath.
		"add": func(L *lua.LState) int {
			tbl := L.CheckTable(1)
			v := L.CheckAny(2)
			if isNone(L, 3) {
				tbl.Append(v)
			} else {
				tbl.Insert(L.CheckInt(3), v)
			}
			L.Push(v)
			return 1
		},

		// del(t, v) takes out the first one equal to v, closing the gap.
		"del": func(L *lua.LState) int {
			tbl := L.CheckTable(1)
			v := L.CheckAny(2)
			for i := 1; i <= tbl.Len(); i++ {
				if tbl.RawGetInt(i) == v {
					tbl.Remove(i)
					L.Push(v)
					return 1
				}
			}
			L.Push(lua.LNil)
			return 1
		},

		// deli(t, i) takes out the one at i, the last by default.
		"deli": func(L *lua.LState) int {
			tbl := L.CheckTable(1)
			i := L.OptInt(2, tbl.Len())
			if i < 1 || i > tbl.Len() {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(tbl.Remove(i))
			return 1
		},

		// all(t) walks the list, and is safe to delete from while walking,
		// which is what makes "for e in all(enemies)" the usual shape of a game
		// loop here.
		"all": func(L *lua.LState) int {
			tbl := L.CheckTable(1)
			i := 0
			L.Push(L.NewFunction(func(L *lua.LState) int {
				for {
					i++
					if i > tbl.Len() {
						L.Push(lua.LNil)
						return 1
					}
					if v := tbl.RawGetInt(i); v != lua.LNil {
						L.Push(v)
						return 1
					}
				}
			}))
			return 1
		},

		"foreach": func(L *lua.LState) int {
			tbl := L.CheckTable(1)
			fn := L.CheckFunction(2)
			for i := 1; i <= tbl.Len(); i++ {
				v := tbl.RawGetInt(i)
				if v == lua.LNil {
					continue
				}
				if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: false}, v); err != nil {
					L.RaiseError("%s", err.Error())
				}
			}
			return 0
		},

		// count(t) is how many things are in the list; count(t, v) is how many
		// of them are v.
		"count": func(L *lua.LState) int {
			tbl := L.CheckTable(1)
			if isNone(L, 2) {
				L.Push(lua.LNumber(tbl.Len()))
				return 1
			}
			want, n := L.Get(2), 0
			for i := 1; i <= tbl.Len(); i++ {
				if tbl.RawGetInt(i) == want {
					n++
				}
			}
			L.Push(lua.LNumber(n))
			return 1
		},

		"sub": func(L *lua.LState) int {
			s := tostr(L.CheckAny(1))
			L.Push(lua.LString(substring(s, L.OptInt(2, 1), L.OptInt(3, len(s)))))
			return 1
		},

		// split("1,2,3") is a table of the numbers 1, 2 and 3: the quickest way
		// to write a level or a tune out as one string.
		"split": func(L *lua.LState) int {
			s := tostr(L.CheckAny(1))
			sep := L.OptString(2, ",")
			convert := L.OptBool(3, true)

			out := L.NewTable()
			var parts []string
			if sep == "" {
				parts = strings.Split(s, "")
			} else {
				parts = strings.Split(s, sep)
			}
			for _, part := range parts {
				if convert {
					if n, err := strconv.ParseFloat(strings.TrimSpace(part), 64); err == nil {
						out.Append(lua.LNumber(n))
						continue
					}
				}
				out.Append(lua.LString(part))
			}
			L.Push(out)
			return 1
		},

		"tostr": func(L *lua.LState) int {
			v := L.Get(1)
			// Only true, or a number that is not zero, asks for hexadecimal.
			//
			// Being strict here would be worse than useless: a call that
			// reports a value and a message hands over both, so
			// tostr(sfx("jump")) would be an error about its second argument.
			// Being merely truthy would be worse still — that message would
			// quietly turn the number into hexadecimal. Anything that is not
			// plainly a flag is not one.
			if wantsHex(L.Get(2)) {
				if n, ok := v.(lua.LNumber); ok {
					L.Push(lua.LString("0x" + strconv.FormatInt(int64(n), 16)))
					return 1
				}
			}
			L.Push(lua.LString(text(L, v)))
			return 1
		},

		// tonum("12") is 12, and tonum of anything that is not a number is nil,
		// which is what makes it usable as a test.
		"tonum": func(L *lua.LState) int {
			switch v := L.Get(1).(type) {
			case lua.LNumber:
				L.Push(v)
			case lua.LString:
				if n, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
					L.Push(lua.LNumber(n))
				} else {
					L.Push(lua.LNil)
				}
			default:
				L.Push(lua.LNil)
			}
			return 1
		},

		"chr": func(L *lua.LState) int {
			var b strings.Builder
			for i := 1; i <= L.GetTop(); i++ {
				b.WriteRune(rune(L.CheckInt(i)))
			}
			L.Push(lua.LString(b.String()))
			return 1
		},

		"ord": func(L *lua.LState) int {
			s := tostr(L.CheckAny(1))
			i := L.OptInt(2, 1)
			runes := []rune(s)
			if i < 1 || i > len(runes) {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(runes[i-1]))
			return 1
		},
	})
}

// reseed restarts the random number generator. It replaces the generator
// rather than reseeding it, because seeding one in place is deprecated and
// this is not a hot path.
func (r *Runtime) reseed(seed int64) {
	r.rng = rand.New(rand.NewSource(seed))
}

// wantsHex reports whether tostr's second argument is asking for hexadecimal.
func wantsHex(v lua.LValue) bool {
	switch v := v.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return v != 0
	}
	return false
}

// num1 wraps a one-number function as a Lua function.
func num1(fn func(float64) float64) lua.LGFunction {
	return func(L *lua.LState) int {
		L.Push(lua.LNumber(fn(float64(L.CheckNumber(1)))))
		return 1
	}
}

// substring takes a slice of a string by character, counting from one, with
// negative positions counting back from the end, as Lua's own string.sub does.
func substring(s string, from, to int) string {
	runes := []rune(s)
	n := len(runes)
	if from < 0 {
		from = n + from + 1
	}
	if to < 0 {
		to = n + to + 1
	}
	from = max(from, 1)
	to = min(to, n)
	if from > to {
		return ""
	}
	return string(runes[from-1 : to])
}

package picolua

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func (r *Runtime) installSystem() {
	r.installSurfaces()

	r.register(map[string]lua.LGFunction{
		// t() and time() are the same thing: seconds since the program started.
		"t":    func(L *lua.LState) int { L.Push(lua.LNumber(r.clock())); return 1 },
		"time": func(L *lua.LState) int { L.Push(lua.LNumber(r.clock())); return 1 },

		// frame() counts ticks, for anything that should happen every so many
		// frames rather than every so many seconds.
		"frame": func(L *lua.LState) int { L.Push(lua.LNumber(r.frame)); return 1 },

		// fps() is what the host is actually managing, which is 0 when nothing
		// is measuring it.
		"fps": func(L *lua.LState) int { L.Push(lua.LNumber(r.fps())); return 1 },

		// printh() writes to the terminal the program was started from. print()
		// draws on the screen, so this is the one to reach for while working
		// something out.
		"printh": func(L *lua.LState) int {
			parts := make([]string, 0, L.GetTop())
			for i := 1; i <= L.GetTop(); i++ {
				parts = append(parts, tostr(L.Get(i)))
			}
			fmt.Fprintln(r.out, strings.Join(parts, "\t"))
			return 0
		},

		// exit([status]) closes the window and ends the program.
		"exit": func(L *lua.LState) int {
			r.quit, r.quitCode = true, L.OptInt(1, 0)
			return 0
		},

		// window{title=, scale=, fullscreen=, width=, height=} asks for the
		// window the program wants. Width and height change the resolution of
		// the screen itself; the rest is up to the host.
		"window": func(L *lua.LState) int {
			opts := L.CheckTable(1)

			if v, ok := opts.RawGetString("title").(lua.LString); ok {
				r.window.Title = string(v)
				r.windowChanged = true
			}
			if v, ok := opts.RawGetString("scale").(lua.LNumber); ok {
				r.window.Scale = int(v)
				r.windowChanged = true
			}
			if v, ok := opts.RawGetString("fullscreen").(lua.LBool); ok {
				r.window.Fullscreen = bool(v)
				r.windowChanged = true
			}

			w, h := r.Vid.Screen.W, r.Vid.Screen.H
			if v, ok := opts.RawGetString("width").(lua.LNumber); ok {
				w = int(v)
			}
			if v, ok := opts.RawGetString("height").(lua.LNumber); ok {
				h = int(v)
			}
			if w != r.Vid.Screen.W || h != r.Vid.Screen.H {
				if w <= 0 || h <= 0 || w*h > maxSurfacePixels {
					L.RaiseError("window: %dx%d is not a size a screen can be", w, h)
				}
				r.Vid.Resize(w, h)
				r.windowChanged = true
			}
			return 0
		},

		// fullscreen() turns it on, fullscreen(false) turns it off.
		"fullscreen": func(L *lua.LState) int {
			r.window.Fullscreen = L.OptBool(1, true)
			r.windowChanged = true
			return 0
		},
	})
}

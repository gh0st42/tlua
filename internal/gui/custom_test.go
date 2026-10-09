package gui

import "testing"

const ratingDef = `
gui.define{
  name = "Rating",
  events = {"onChange", "rated"},
  build = function(parent, opts)
    local c = parent:Canvas{width = 100, height = 20}
    c.value = opts.value or 0
    function c:set(v)
      self.value = v
      self:fire("onChange", v)
    end
    return c
  end,
}`

func TestDefinedControls(t *testing.T) {
	L, _ := newState(t)
	run(t, L, ratingDef)
	run(t, L, `
form = gui.Form{}
frame = form:Frame{}
r = frame:Rating{value = 3, left = 5, top = 6, width = 150, onChange = function(self, v) got = v; who = self end}
assert(r.value == 3 and r.left == 5 and r.top == 6 and r.width == 150 and r.height == 20)
assert(tostring(r) == "Rating")
r:set(4)
assert(got == 4 and who == r, "fire calls the handler with self")
r.onRated = function(self, n) return n * 2 end
assert(r:fire("rated", 21) == 42, "fire returns what the handler does, and takes either name")
loose = gui.Rating{}
assert(loose.value == 0)`)
	if global(t, L, "r").parent != global(t, L, "frame") {
		t.Fatal("frame:Rating{} should build inside the frame")
	}
	if global(t, L, "loose").parent != global(t, L, "form") {
		t.Fatal("gui.Rating{} should join the latest form")
	}

	fails(t, L, `r.onClick = print`, "a Rating has no event onClick (it has onChange, onRated, onDraw")
	fails(t, L, `r:fire("click")`, "a Rating has no event onClick")
	fails(t, L, `form:Tabs{}:Rating{}`, "a Tabs cannot hold a Rating")
	fails(t, L, `r.Rating = 1`, "Rating is a method")
	run(t, L, `form:Button{}:fire("click")`) // a built-in event, no handler: nothing happens
}

func TestDefineChecksItsInput(t *testing.T) {
	L, _ := newState(t)
	run(t, L, ratingDef)
	fails(t, L, `gui.define{name = "Rating", build = print}`, "Rating is already defined")
	fails(t, L, `gui.define{name = "Button", build = print}`, "Button is one of the built-in kinds")
	fails(t, L, `gui.define{name = "rating", build = print}`, "should start with a capital")
	fails(t, L, `gui.define{name = "Two Words", build = print}`, "is not a name")
	fails(t, L, `gui.define{name = "Nothing"}`, "build must be a function")
	fails(t, L, `gui.define{name = "Odd", events = {""}, build = print}`, "is not an event name")
	run(t, L, `gui.define{name = "Broken", build = function() return 42 end}`)
	fails(t, L, `gui.Form{}:Broken{}`, "the build of Broken must return the control it made")

	L2, _ := newState(t)
	run(t, L2, ratingDef)
	fails(t, L2, `gui.Rating{}`, "gui.Rating needs somewhere to go")
}

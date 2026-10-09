#!/usr/bin/env tlua

-- Controls of your own: gui.define gives a name to a function that builds a
-- control out of the built-in ones. Rating is drawn on a Canvas; ColorPicker
-- is a Frame of other controls. Both raise an event of their own.

local gui = bootgui()

-- A row of stars to click. Everything it shows comes from its properties,
-- and a Canvas redraws when one is assigned, so rating.value = 3 is all it
-- takes to change it from outside.
gui.define {
  name = "Rating",
  events = { "onChange" },
  build = function(parent, opts)
    local size = 24
    local c = parent:Canvas { width = (opts.max or 5) * size, height = size }
    c.max, c.value, c.hover = opts.max or 5, opts.value or 0, 0

    local function star(g, cx, cy, r)
      -- Five points and five dents, filled as triangles from the middle,
      -- since a star is not convex.
      local pts = {}
      for i = 0, 9 do
        local a = math.pi / 2 + i * math.pi / 5
        local d = (i % 2 == 0) and r or r * 0.45
        pts[#pts + 1] = { cx + d * math.cos(a), cy - d * math.sin(a) }
      end
      for i = 1, 10 do
        local p, q = pts[i], pts[i % 10 + 1]
        g:polygon(cx, cy, p[1], p[2], q[1], q[2])
      end
    end

    function c:onDraw(g)
      local lit = self.hover > 0 and self.hover or self.value
      for i = 1, self.max do
        g:color(i <= lit and "#f0b400" or "#d0d0d0")
        star(g, (i - 0.5) * size, size / 2, size / 2 - 2)
      end
    end

    local function starAt(self, x)
      return math.max(1, math.min(self.max, math.floor(x / size) + 1))
    end
    function c:onMouseMove(x) self.hover = starAt(self, x) end
    function c:onMouseLeave() self.hover = 0 end
    function c:onMouseDown(x)
      self.value = starAt(self, x)
      self:fire("onChange", self.value)
    end
    return c
  end,
}

-- Three sliders and a swatch, in a frame. Its value is a colour as
-- "#rrggbb", and set() changes it from outside.
gui.define {
  name = "ColorPicker",
  events = { "onChange" },
  build = function(parent, opts)
    local f = parent:Frame { caption = opts.caption or "Colour", width = 260, height = 132 }
    local swatch = f:Canvas { left = 194, top = 26, width = 54, height = 94 }
    local sliders = {}

    local function changed()
      f.value = string.format("#%02x%02x%02x", sliders[1].value, sliders[2].value, sliders[3].value)
      swatch.color = f.value
      f:fire("onChange", f.value)
    end
    for i, name in ipairs { "R", "G", "B" } do
      local top = 26 + (i - 1) * 32
      f:Label { caption = name, left = 10, top = top, width = 20 }
      sliders[i] = f:Slider { left = 30, top = top, width = 156, max = 255, onChange = changed }
    end

    function f:set(color)
      local r, g, b = color:match("^#(%x%x)(%x%x)(%x%x)$")
      assert(r, "a colour is #rrggbb")
      sliders[1].value, sliders[2].value, sliders[3].value = tonumber(r, 16), tonumber(g, 16), tonumber(b, 16)
      f.value = color
      swatch.color = color
    end
    f:set(opts.value or "#4080c0")
    return f
  end,
}

-- Used like any other control.
local form = gui.Form { caption = "Custom controls", width = 300, height = 260 }

form:Label { caption = "How useful is this?", left = 16, top = 16, width = 268 }
local said = form:Label { caption = "3 stars", left = 150, top = 44, width = 130 }
local stars = form:Rating {
  left = 16, top = 44, value = 3,
  onChange = function(self, n) said.caption = n .. (n == 1 and " star" or " stars") end,
}

local picker = form:ColorPicker {
  caption = "Background", left = 16, top = 84, value = "#fff4dc",
  onChange = function(self, color) form.color = color end,
}
form.color = picker.value

form:Button {
  caption = "Reset", left = 16, top = 224, width = 100,
  onClick = function()
    stars.value = 0
    said.caption = "not rated"
    picker:set("#ffffff")
    form.color = picker.value
  end,
}

form:show()

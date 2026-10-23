-- Three sliders and a swatch. self.rgb is the colour as "#rrggbb", self:set
-- changes it, and onChange(self, rgb) is raised when a slider is moved.

---@param self components.RgbPicker
---@param parts components.RgbPicker.parts
return function(self, parts)
  local function whole(v) return math.floor(v + 0.5) end

  local function show(fire)
    self.rgb = ("#%02x%02x%02x"):format(whole(parts.r.value), whole(parts.g.value), whole(parts.b.value))
    parts.swatch.color = self.rgb
    parts.hex.caption = self.rgb
    if fire then self:fire("onChange", self.rgb) end
  end

  for _, slider in ipairs { parts.r, parts.g, parts.b } do
    slider.onChange = function() show(true) end
  end

  function self:set(rgb)
    local r, g, b = rgb:match("^#(%x%x)(%x%x)(%x%x)$")
    if not r then error("RgbPicker:set wants \"#rrggbb\", not " .. tostring(rgb), 2) end
    parts.r.value, parts.g.value, parts.b.value = tonumber(r, 16), tonumber(g, 16), tonumber(b, 16)
    show(false)
  end

  show(false)
end

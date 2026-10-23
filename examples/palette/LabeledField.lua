-- A label and a box that go together. label and text are those of the two
-- parts; onChange is the box's.

---@param self components.LabeledField
---@param parts components.LabeledField.parts
return function(self, parts)
  function self:clear() parts.txt.text = "" end
end

-- A box with a button: onSearch(self, text) is raised by the button and by
-- Enter in the box.

---@param self components.SearchBox
---@param parts components.SearchBox.parts
return function(self, parts)
  local function search() self:fire("onSearch", parts.txt.text) end
  parts.go.onClick = search
  function parts.txt:onKey(key)
    if key == "Enter" then search(); return true end
  end
end

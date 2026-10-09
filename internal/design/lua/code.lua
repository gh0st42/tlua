-- What the designer does to a form's code: find its handlers, add new ones,
-- rename the controls they mention, and read where an error happened. All of
-- it is text in and text out, so it is tested without a screen.
--
-- The code is the programmer's. The designer only adds a handler where it is
-- asked for one, before the `return frm` at the end, and renames only when
-- the programmer agrees to it.

local M = {}

-- The event VB6's double-click went to, for each kind.
local defaults = {
  Button = "onClick", TextBox = "onChange", CheckBox = "onChange",
  RadioButton = "onChange", ComboBox = "onChange", ListBox = "onChange",
  Tree = "onChange", Table = "onChange", Slider = "onChange", Spinner = "onChange",
  Tabs = "onChange", Canvas = "onDraw", Form = "onClose", Menu = "onClick",
}

function M.defaultEvent(kind)
  return defaults[kind]
end

-- What each handler is given, after self.
local params = {
  onKey = "key, text", onDrop = "text, lines", onDraw = "g",
  onMouseDown = "x, y, button, double", onMouseUp = "x, y, button",
  onMouseMove = "x, y", onMouseDrag = "x, y", onMouseWheel = "dx, dy",
  onToggle = "path, open", onContextMenu = "name, caption, checked",
}

-- A Menu's onClick says which item; a Button's says nothing.
local kindParams = { Menu = { onClick = "name, caption, checked" } }

function M.params(event, kind)
  return kindParams[kind] and kindParams[kind][event] or params[event] or ""
end

local function escape(s)
  return (s:gsub("[%^%$%(%)%%%.%[%]%*%+%-%?]", "%%%0"))
end

-- formVar is what the code calls its form: the local it loads the layout
-- into, frm unless the programmer named it otherwise.
function M.formVar(text)
  return text:match("local%s+([%a_][%w_]*)%s*=%s*gui%.load") or "frm"
end

-- target is how a handler's owner is written: frm.Button1, or frm for the
-- form itself.
local function target(var, name)
  if name then return var .. "." .. name end
  return var
end

-- lineAt is the line number of a byte position.
local function lineAt(text, pos)
  local _, n = text:sub(1, pos - 1):gsub("\n", "")
  return n + 1
end

-- findHandler is the line a handler starts on, written either way a
-- handler can be, or nil.
function M.findHandler(text, var, name, event)
  local t = escape(target(var, name))
  local e = escape(event)
  for _, pattern in ipairs {
    "function%s+" .. t .. ":" .. e .. "%s*%(",
    t .. "%." .. e .. "%s*=%s*function",
  } do
    local at = text:find(pattern)
    if at then return lineAt(text, at) end
  end
end

-- addHandler adds an empty handler, before the line that returns the form
-- if there is one, and returns the new text and the line inside the
-- handler, where the programmer starts typing.
function M.addHandler(text, var, name, event, kind)
  local lines = {}
  for line in (text .. "\n"):gmatch("(.-)\n") do lines[#lines + 1] = line end
  while lines[#lines] == "" do lines[#lines] = nil end
  local at = #lines + 1
  for i = #lines, 1, -1 do
    if lines[i]:match("^%s*return%s+" .. escape(var) .. "%s*$") then
      at = i
      break
    end
  end
  local out = {}
  for i = 1, at - 1 do out[#out + 1] = lines[i] end
  -- A blank line on either side of it.
  if #out > 0 and out[#out] ~= "" then out[#out + 1] = "" end
  local start = #out + 1
  out[#out + 1] = ("function %s:%s(%s)"):format(target(var, name), event, M.params(event, kind))
  out[#out + 1] = "  "
  out[#out + 1] = "end"
  if at <= #lines then
    out[#out + 1] = ""
    for i = at, #lines do out[#out + 1] = lines[i] end
  end
  return table.concat(out, "\n") .. "\n", start + 1
end

-- countRefs and renameRefs find and change what the code calls a control:
-- frm.Old, but not frm.Older.
local function refPattern(var, old)
  return "(" .. escape(var) .. "%.)" .. escape(old) .. "([^%w_])"
end

function M.countRefs(text, var, old)
  local _, n = (text .. "\n"):gsub(refPattern(var, old), "")
  return n
end

function M.renameRefs(text, var, old, new)
  local out, n = (text .. "\n"):gsub(refPattern(var, old), "%1" .. new:gsub("%%", "%%%%") .. "%2")
  return out:sub(1, -2), n
end

-- errorAt reads where an error message says it happened: the form whose
-- code it was in, and the line, or nil.
function M.errorAt(message)
  for file, line in message:gmatch("([%w_%.%-/\\]+%.lua):(%d+):") do
    local form = file:match("forms[/\\]([%a_][%w_]*)%.lua$")
    if form and not file:match("%.form%.lua$") then return form, tonumber(line) end
  end
end

return M

#!/usr/bin/env tlua
-- A help window: Markdown pages in help/, shown by a MarkdownView, with a
-- list of the page's headings beside it, back and forward buttons, and a
-- search box. Links between the pages open in the same view; web links go
-- to the browser; an app: link is the program's own to answer.

local gui = bootgui()

local frm = gui.Form { caption = "Help", width = 760, height = 520, resizable = true }
local back = frm:Button { caption = "<", left = 8, top = 8, width = 32, enabled = false }
local fwd = frm:Button { caption = ">", left = 44, top = 8, width = 32, enabled = false }
local find = frm:TextBox { left = 560, top = 8, width = 192, tooltip = "Find on this page (Enter)" }
local status = frm:Label { left = 84, top = 8, width = 470, textColor = "#707070" }
local toc = frm:ListBox { left = 8, top = 44, width = 180, height = 468 }
local view = frm:MarkdownView { left = 196, top = 44, width = 556, height = 468, file = "help/index.md" }

local anchors = {}
local function refresh()
  back.enabled, fwd.enabled = view:canGoBack(), view:canGoForward()
  local items = {}
  anchors = {}
  for _, h in ipairs(view:headings()) do
    items[#items + 1] = ("  "):rep(h.level - 1) .. h.text
    anchors[#anchors + 1] = h.anchor
  end
  toc.items = items
  frm.caption = "Help - " .. (view.file:match("[^/\\]+$") or "")
end

function view:onNavigate() refresh() end
function view:onHover(url) status.caption = url or "" end
function view:onLink(url)
  if url:match("^app:") then
    gui.msgbox("The program would " .. url:sub(5) .. " here.", "ok", "Help")
    return true -- handled: the view does nothing more
  end
end

function back:onClick() view:back() end
function fwd:onClick() view:forward() end
function toc:onChange()
  if anchors[toc.selected] then view:scrollTo(anchors[toc.selected]) end
end
function find:onKey(key)
  if key == "Enter" then
    if not view:search(find.text) then status.caption = "Not found: " .. find.text end
    return true
  end
end

function frm:onResize(w, h)
  find.left = w - 200
  toc.height = h - 52
  view.width, view.height = w - 204, h - 52
end

refresh()
frm:show()

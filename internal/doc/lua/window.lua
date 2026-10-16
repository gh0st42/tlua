-- The window `tlua doc -g` opens: a page in a MarkdownView, the headings
-- of it beside it, back, forward and contents buttons, and a search box.
-- Links between the pages open in the same view, web links in the browser.

local gui = require("gui")

local M = {}

-- open shows file (scrolled to anchor) and returns when the window is
-- closed; index is the contents page.
function M.open(file, anchor, index)
    local frm = gui.Form { caption = "tlua help", width = 900, height = 640, resizable = true }
    local back = frm:Button { caption = "<", left = 8, top = 8, width = 32, enabled = false, tooltip = "Back" }
    local fwd = frm:Button { caption = ">", left = 44, top = 8, width = 32, enabled = false, tooltip = "Forward" }
    local home = frm:Button { caption = "Contents", left = 84, top = 8, width = 80 }
    local status = frm:Label { left = 172, top = 8, width = 520, textColor = "#707070" }
    local find = frm:TextBox { left = 700, top = 8, width = 192, tooltip = "Find on this page (Enter)" }
    local toc = frm:ListBox { left = 8, top = 44, width = 200, height = 588 }
    local view = frm:MarkdownView { left = 216, top = 44, width = 676, height = 588, file = file }

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
        local title = view:headings()[1]
        frm.caption = "tlua help - " .. (title and title.text or view.file:match("[^/\\]+$") or "")
        status.caption = ""
    end

    function view:onNavigate() refresh() end
    function view:onHover(url) status.caption = url or "" end
    function view:onLink(url)
        if url:match("^%a[%w+.-]*:") then return end -- the web: the view opens it
        local path = url:match("^([^#]*)")
        if path ~= "" and not path:match("%.md$") then
            -- An example or a source file: in the repository, not the help.
            status.caption = url .. " is in tlua's repository, not in the help"
            return true
        end
    end

    function back:onClick() view:back() end
    function fwd:onClick() view:forward() end
    function home:onClick() view:open(index) end
    function toc:onChange()
        if anchors[toc.selected] then view:scrollTo(anchors[toc.selected]) end
    end
    function find:onKey(key)
        if key == "Enter" then
            if not view:search(find.text) then status.caption = "Not found: " .. find.text end
            return true
        end
    end

    function frm:onResize()
        local w, h = frm.width, frm.height
        find.left = w - 200
        status.width = w - 380
        toc.height = h - 52
        view.width, view.height = w - 224, h - 52
    end

    refresh()
    if anchor then view:scrollTo(anchor) end
    frm:show()
end

return M

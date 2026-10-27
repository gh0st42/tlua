-- doc: what `tlua doc` shows. Finds a topic among the pages (a page by its
-- name, or a heading in one, such as sys.copy) and draws Markdown for a
-- terminal: headings, wrapped paragraphs, lists, code and tables, with ANSI
-- colours when the output is a terminal.

local md = require("tlua.markdown")

local M = {}

-- Other names a page goes by.
local aliases = {
    lua = "lua-language", language = "lua-language", stdlib = "lua-stdlib",
    modules = "index", contents = "index", help = "index",
}

-- The width of a UTF-8 string on the screen, a character a column.
local function ulen(s)
    return #s:gsub("[\128-\191]", "")
end

local function plain(runs)
    local out = {}
    for _, r in ipairs(runs or {}) do out[#out + 1] = r.text end
    return table.concat(out)
end

-- The page a name means, and the blocks it is made of.
local function page(pages, name)
    name = aliases[name] or name
    if pages[name] then
        return name, md.parse(pages[name])
    end
end

-- The heading of blocks whose text has topic in it, as markdown.headings
-- gives it.
local function heading(blocks, topic)
    for _, h in ipairs(md.headings(blocks)) do
        if h.text:lower():find(topic, 1, true) then return h end
    end
end

-- find says what a topic means: a page name and, for a heading in it, the
-- heading; or nil and the headings of every page that mention it.
function M.find(pages, topic)
    topic = topic:lower()
    local name, blocks = page(pages, topic)
    if name then return name, nil end
    -- page.function, page.Class:method, page.sub.function: the longest
    -- page name it starts with, then the longest heading that matches.
    local prefix = topic
    while true do
        local cut = prefix:match("^(.*)[.:][^.:]*$")
        if not cut then break end
        prefix = cut
        name, blocks = page(pages, prefix)
        if name then
            -- The page may write its module shorter: socket.http's
            -- functions are http.request and the like.
            local short = prefix:match("[^.]*$")
            local want = topic
            while #want > #prefix do
                local h = heading(blocks, want) or heading(blocks, short .. want:sub(#prefix + 1))
                -- Or a method: png.rotate is the heading img:rotate(turns).
                if not h and want:sub(#prefix + 1, #prefix + 1) == "." and #want > #prefix + 1 then
                    h = heading(blocks, ":" .. want:sub(#prefix + 2))
                end
                if h then return name, h end
                want = want:match("^(.*)[.:][^.:]*$") or prefix
            end
            return name, nil
        end
    end
    -- Anywhere.
    local found = {}
    local names = {}
    for n in pairs(pages) do names[#names + 1] = n end
    table.sort(names)
    for _, n in ipairs(names) do
        for _, h in ipairs(md.headings(md.parse(pages[n]))) do
            if h.text:lower():find(topic, 1, true) then
                found[#found + 1] = { page = n, heading = h }
            end
        end
    end
    if #found == 1 then return found[1].page, found[1].heading end
    return nil, found
end

-- Styles, as ANSI sequences; none when colour is off.
local styles = {
    b = "\27[1m", i = "\27[3m", code = "\27[36m", link = "\27[4m",
    h1 = "\27[1;33m", h2 = "\27[1;36m", h3 = "\27[1m", h = "\27[1;2m",
    quote = "\27[2m", pre = "\27[32m", dim = "\27[2m", head = "\27[1m",
}
local reset = "\27[0m"

-- words turns runs into words, each with its style, and "\n" for a line
-- break.
local function words(runs, color, extra)
    local out = {}
    for _, r in ipairs(runs or {}) do
        local style = ""
        if color then
            style = (extra or "") .. (r.b and styles.b or "") .. (r.i and styles.i or "")
                .. (r.code and styles.code or "") .. (r.link and styles.link or "")
        end
        local text = r.text
        if r.link and r.link:match("^https?:") and r.link ~= r.text then
            text = text .. " <" .. r.link .. ">"
        end
        local first = true
        for line in (text .. "\n"):gmatch("(.-)\n") do
            if not first then out[#out + 1] = { br = true } end
            first = false
            local at = 1
            while at <= #line do
                local s, e = line:find("%s+", at)
                local word = line:sub(at, (s or #line + 1) - 1)
                if word ~= "" then
                    out[#out + 1] = { text = word, style = style }
                end
                if s then
                    out[#out + 1] = { space = true }
                    at = e + 1
                else
                    break
                end
            end
        end
    end
    return out
end

-- wrap lays words out in lines of at most width columns, the first after
-- lead, the others after indent.
local function wrap(ws, width, lead, indent, color)
    -- Pieces with no space between them (`a`/`b`, **x**y) are one word.
    local units, unit = {}, nil
    for _, w in ipairs(ws) do
        if w.br or w.space then
            unit = nil
            units[#units + 1] = w
        else
            if not unit then
                unit = { parts = {}, n = 0 }
                units[#units + 1] = unit
            end
            unit.parts[#unit.parts + 1] = (color and w.style ~= "") and (w.style .. w.text .. reset) or w.text
            unit.n = unit.n + ulen(w.text)
        end
    end
    local lines, line, used = {}, { lead }, ulen(lead)
    local words, space = 0, false
    local function flush()
        lines[#lines + 1] = table.concat(line)
        line, used, words, space = { indent }, ulen(indent), 0, false
    end
    for _, u in ipairs(units) do
        if u.br then
            flush()
        elseif u.space then
            space = words > 0
        else
            if space and used + 1 + u.n > width then
                flush()
            elseif space then
                line[#line + 1] = " "
                used = used + 1
            end
            space = false
            line[#line + 1] = table.concat(u.parts)
            used, words = used + u.n, words + 1
        end
    end
    lines[#lines + 1] = table.concat(line)
    return lines
end

local function pad(s, w, align)
    local gap = w - ulen(s)
    if gap <= 0 then return s end
    if align == "right" then return (" "):rep(gap) .. s end
    if align == "center" then
        local l = math.floor(gap / 2)
        return (" "):rep(l) .. s .. (" "):rep(gap - l)
    end
    return s .. (" "):rep(gap)
end

-- A cell's text broken into lines of at most w columns.
local function cellLines(text, w)
    local out, line = {}, ""
    for word in text:gmatch("%S+") do
        if line == "" then
            line = word
        elseif ulen(line) + 1 + ulen(word) <= w then
            line = line .. " " .. word
        else
            out[#out + 1] = line
            line = word
        end
        while ulen(line) > w do
            out[#out + 1] = line:sub(1, w)
            line = line:sub(w + 1)
        end
    end
    out[#out + 1] = line
    return out
end

local function drawTable(b, width, color)
    local rows, n = {}, 0
    for r, row in ipairs(b.rows) do
        rows[r] = {}
        for c, cell in ipairs(row) do
            rows[r][c] = plain(cell)
            if c > n then n = c end
        end
    end
    local widths = {}
    for c = 1, n do
        widths[c] = 1
        for r = 1, #rows do
            widths[c] = math.max(widths[c], ulen(rows[r][c] or ""))
        end
    end
    -- Narrow the widest columns until the table fits.
    local function total()
        local t = 3 * (n - 1)
        for c = 1, n do t = t + widths[c] end
        return t
    end
    while total() > width do
        local widest = 1
        for c = 2, n do if widths[c] > widths[widest] then widest = c end end
        if widths[widest] <= 8 then break end
        widths[widest] = widths[widest] - 1
    end
    local bar = color and " │ " or " | "
    local out = {}
    for r = 1, #rows do
        local cells, height = {}, 1
        for c = 1, n do
            cells[c] = cellLines(rows[r][c] or "", widths[c])
            height = math.max(height, #cells[c])
        end
        for l = 1, height do
            local parts = {}
            for c = 1, n do
                local text = pad(cells[c][l] or "", widths[c], b.align and b.align[c])
                if r == 1 and color then text = styles.head .. text .. reset end
                parts[c] = text
            end
            out[#out + 1] = (table.concat(parts, bar):gsub("%s+$", ""))
        end
        if r == 1 then
            local parts = {}
            for c = 1, n do parts[c] = (color and "─" or "-"):rep(widths[c]) end
            out[#out + 1] = table.concat(parts, color and "─┼─" or "-+-")
        end
    end
    return out
end

-- render draws blocks, from the first to last, as lines of text.
function M.render(blocks, opts)
    local width, color = opts.width or 80, opts.color
    local first, last = opts.first or 1, opts.last or #blocks
    local out = {}
    local function add(lines)
        for _, l in ipairs(lines) do out[#out + 1] = l end
    end
    local numbers = {}
    local prev
    for i = first, last do
        local b = blocks[i]
        local t = b.type
        local list = t == "ul" or t == "ol"
        if prev and not (list and (prev == "ul" or prev == "ol")) then
            out[#out + 1] = ""
        end
        if t ~= "ol" then numbers = {} end
        local level = tonumber(t:match("^h(%d)$"))
        if level then
            local text = plain(b.runs)
            if color then
                local style = styles["h" .. level] or styles.h
                out[#out + 1] = style .. text .. reset
            else
                out[#out + 1] = text
                if level <= 2 then
                    out[#out + 1] = (level == 1 and "=" or "-"):rep(math.min(ulen(text), width))
                end
            end
        elseif t == "p" then
            add(wrap(words(b.runs, color), width, "", "", color))
        elseif list then
            local lvl = b.level or 0
            local indent = ("  "):rep(lvl)
            local mark
            if t == "ol" then
                for deeper = lvl + 1, 4 do numbers[deeper] = nil end
                numbers[lvl] = (numbers[lvl] or 0) + 1
                mark = numbers[lvl] .. ". "
            else
                numbers = {}
                mark = color and "• " or "- "
            end
            add(wrap(words(b.runs, color), width, indent .. mark, indent .. (" "):rep(ulen(mark)), color))
        elseif t == "quote" then
            local bar = color and (styles.quote .. "│" .. reset .. " ") or "> "
            add(wrap(words(b.runs, color, color and styles.quote), width, bar, bar, color))
        elseif t == "code" then
            for line in (plain(b.runs):gsub("\n$", "") .. "\n"):gmatch("(.-)\n") do
                out[#out + 1] = "    " .. (color and (styles.pre .. line .. reset) or line)
            end
        elseif t == "table" then
            add(drawTable(b, width, color))
        elseif t == "hr" then
            out[#out + 1] = (color and "─" or "-"):rep(width)
        elseif t == "image" then
            local text = "[image: " .. (b.alt or b.src or "") .. "]"
            out[#out + 1] = color and (styles.dim .. text .. reset) or text
        elseif t == "toc" then
            for _, h in ipairs(md.headings(blocks)) do
                if h.level > 1 then
                    out[#out + 1] = ("  "):rep(h.level - 2) .. (color and "• " or "- ") .. h.text
                end
            end
        end
        prev = t
    end
    return table.concat(out, "\n") .. "\n"
end

-- section is where a heading's part of a page ends: before the next
-- heading of its level or above.
local function section(blocks, h)
    for i = h.block + 1, #blocks do
        local level = tonumber(blocks[i].type:match("^h(%d)$"))
        if level and level <= h.level then return i - 1 end
    end
    return #blocks
end

-- names are the pages, sorted, index first.
local function names(pages)
    local list = {}
    for n in pairs(pages) do
        if n ~= "index" then list[#list + 1] = n end
    end
    table.sort(list)
    table.insert(list, 1, "index")
    return list
end

-- show is `tlua doc [topic]`: the text to show, or nil and why.
function M.show(pages, topic, opts)
    if topic == nil or topic == "" then
        local text = M.render(md.parse(pages.index), opts)
        return text .. "\nPages: " .. table.concat(names(pages), ", ") .. "\n"
            .. "tlua doc <page> shows one, tlua doc <page>.<name> a part of one\n"
            .. "(tlua doc sys.copy), and tlua doc -g [topic] opens a window.\n"
    end
    local name, found = M.find(pages, topic)
    if not name then
        if #found == 0 then
            return nil, ("nothing about %q; the pages are %s"):format(topic, table.concat(names(pages), ", "))
        end
        local lines = { ("%q is in more than one place:"):format(topic) }
        for _, f in ipairs(found) do
            lines[#lines + 1] = ("  tlua doc %s.%s  (%s)"):format(f.page, topic, f.heading.text)
        end
        return nil, table.concat(lines, "\n")
    end
    local blocks = md.parse(pages[name])
    if found then
        opts.first, opts.last = found.block, section(blocks, found)
    end
    return M.render(blocks, opts)
end

-- target is what the window opens for a topic: a page's file and the
-- heading's anchor, or nil and why.
function M.target(pages, files, topic)
    if topic == nil or topic == "" then return files.index end
    local name, found = M.find(pages, topic)
    if not name then
        return nil, ("nothing about %q, or it is in more than one place; try tlua doc %s"):format(topic, topic)
    end
    return files[name], found and found.anchor
end

return M

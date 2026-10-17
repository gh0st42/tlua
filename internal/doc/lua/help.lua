-- help: the documentation's window for a topic, as `tlua doc -g` and the
-- designer's Help menu open it. The pages come from tlua.docpages: their
-- texts, and their files written out to the user's cache.

local doc = require("tlua.doc")
local window = require("tlua.window")

local M = {}

-- open shows the page for topic (the contents for none) and says whether
-- it found it. When it did not, strict leaves it at that, and otherwise the
-- contents are shown instead.
function M.open(topic, strict)
    local pages = require("tlua.docpages")
    local file, anchor = doc.target(pages.texts, pages.files, topic)
    if not file then
        if strict then return nil, anchor end
        window.open(pages.files.index, nil, pages.files.index)
        return nil, anchor
    end
    window.open(file, anchor, pages.files.index)
    return true
end

return M

-- Loaded through the "<dir>/?/init.lua" half of package.path.
local util = {}

function util.map(t, f)
  local out = {}
  for i, v in ipairs(t) do out[i] = f(v) end
  return out
end

function util.sum(t)
  local n = 0
  for _, v in ipairs(t) do n = n + v end
  return n
end

return util

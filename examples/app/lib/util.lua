local util = {}

function util.sum(t)
  local n = 0
  for _, v in ipairs(t) do n = n + v end
  return n
end

return util

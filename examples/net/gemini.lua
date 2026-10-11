#!/usr/bin/env tlua
-- A Gemini client: fetches a gemini:// page and prints it.
--
--   tlua examples/net/gemini.lua gemini://geminiprotocol.net/ [known_hosts]
--
-- Gemini runs over TLS, usually with a certificate the capsule signed
-- itself, so there is no authority to check it against. Instead a client
-- trusts a host's certificate the first time it sees it and expects the
-- same one after that ("trust on first use"): give a file as the second
-- argument and the fingerprints are kept there.

local socket = require "socket"
local ssl = require "ssl"
local url = require "socket.url"

local target, knownFile = arg[1], arg[2]
if not target then
  io.stderr:write("usage: tlua gemini.lua gemini://host/path [known_hosts]\n")
  os.exit(1)
end

-- known holds the fingerprint each host showed first, "host:port fingerprint" a line.
local known = {}
if knownFile then
  local f = io.open(knownFile)
  if f then
    for line in f:lines() do
      local host, fp = line:match("^(%S+)%s+(%x+)$")
      if host then known[host] = fp end
    end
    f:close()
  end
end

local function remember(host, fp)
  known[host] = fp
  if knownFile then
    local f = assert(io.open(knownFile, "a"))
    f:write(host, " ", fp, "\n")
    f:close()
  end
end

-- fetch makes one request and returns the status, the meta line and the body.
local function fetch(address)
  local u = url.parse(address)
  if u.scheme ~= "gemini" then return nil, "not a gemini:// address: " .. address end
  local port = tonumber(u.port) or 1965
  local tcp = assert(socket.tcp())
  tcp:settimeout(15)
  local ok, err = tcp:connect(u.host, port)
  if not ok then return nil, "connecting to " .. u.host .. ": " .. err end

  local conn = assert(ssl.wrap(tcp, { mode = "client", protocol = "tlsv1_2", options = "all" }))
  conn:sni(u.host) -- many capsules share an address and tell themselves apart by name
  conn:settimeout(15)
  ok, err = conn:dohandshake()
  if not ok then return nil, "TLS with " .. u.host .. ": " .. err end

  -- Trust on first use.
  local key = u.host .. ":" .. port
  local fp = conn:getpeercertificate():digest("sha256")
  if not known[key] then
    remember(key, fp)
    io.stderr:write(("[first visit to %s: trusting certificate %s...]\n"):format(key, fp:sub(1, 16)))
  elseif known[key] ~= fp then
    conn:close()
    return nil, key .. " shows a certificate it did not show before; refusing it"
  end

  assert(conn:send(address .. "\r\n"))
  local header, herr = conn:receive("*l")
  if not header then return nil, "no answer: " .. tostring(herr) end
  local status, meta = header:match("^(%d%d)%s*(.*)$")
  local body = conn:receive("*a") or ""
  conn:close()
  return tonumber(status), meta, body
end

local address = target
for _ = 1, 5 do -- redirects
  local status, meta, body = fetch(address)
  if not status then
    io.stderr:write(meta, "\n")
    os.exit(1)
  end
  if status >= 30 and status < 40 then
    address = url.absolute(address, meta)
    io.stderr:write("[redirected to ", address, "]\n")
  elseif status >= 20 and status < 30 then
    io.write(body)
    os.exit(0)
  elseif status >= 10 and status < 20 then
    io.stderr:write("the capsule asks for input: ", meta, "\n")
    os.exit(1)
  else
    io.stderr:write(("error %d: %s\n"):format(status, meta))
    os.exit(1)
  end
end
io.stderr:write("too many redirects\n")
os.exit(1)

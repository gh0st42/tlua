-- Entry point of a fused app: `tlua fuse -o demo examples/app && ./demo`
local util = require("lib.util")   -- resolved from inside the archive
local embed = require("embed")     -- the archive itself

print("argv[0]:", arg[0])
print("args:", ...)
print("sum:", util.sum({1, 2, 3, 4}))
print("banner:", (embed.read("data/banner.txt"):gsub("%s+$", "")))
print("bundled files:", table.concat(embed.files(), " "))

local cfg = dofile("data/config.lua")  -- read from the archive, not the cwd
print("config:", cfg.name, cfg.version)

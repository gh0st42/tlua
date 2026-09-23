#!/usr/bin/env tlua
-- A plain Lua 5.1 script: requires siblings from disk, uses the stdlib.
local greet = require("lib.greet")
local util = require("lib.util")

print(greet.hello(arg[1]))
print("squares:", table.concat(util.map({1, 2, 3, 4}, function(n) return n * n end), ", "))
print("sum:", util.sum({1, 2, 3, 4}))
print("lua:", _VERSION, "os:", type(os.time()) == "number" and "ok" or "?")

local f = assert(io.open(os.tmpname(), "w+"))
f:write("round trip\n")
f:seek("set", 0)
io.write("io: ", f:read("*l"), "\n")
f:close()

local ok, err = pcall(function() error("caught: boom") end)
print("pcall:", ok, err)

for word in ("coroutines and metatables"):gmatch("%a+") do io.write(word, " ") end
print()

local co = coroutine.create(function(a) local b = coroutine.yield(a + 1) return b * 2 end)
local _, v = coroutine.resume(co, 1)
local _, w = coroutine.resume(co, v)
print("coroutine:", v, w)

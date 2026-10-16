-- sys: files, folders and a few system helpers, from turbolua

local sys = {}

local DIR_SEP = package.config:sub(1, 1)

-- lfs is built into tlua, so the fallbacks turbolua had for a Lua without
-- it (ls, mkdir and pwd run in a shell) are not needed.
local lfs = require("lfs")

local function normalize_path(path)
    if type(path) ~= "string" or path == "" then
        return nil, "path must be a non-empty string"
    end
    return path
end

local function dirname(path)
    local p = path:gsub("[\\/]$", "")
    local idx = p:match("^.*()[/\\]")
    if not idx then
        return nil
    end
    if idx == 1 then
        return p:sub(1, 1)
    end
    return p:sub(1, idx - 1)
end

local function stat(path)
    return lfs.attributes(path)
end

local function command_ok(ok, why, code)
    if ok == true then
        return true
    end
    if type(ok) == "number" then
        return ok == 0
    end
    if why == "exit" then
        return code == 0
    end
    return false
end

local function mkdir_one(path)
    if sys.exists(path) then
        if sys.isdir(path) then
            return true
        end
        return nil, "path exists but is not a directory: " .. path
    end

    local ok, err = lfs.mkdir(path)
    if ok then
        return true
    end
    return nil, err or ("failed to create directory: " .. path)
end

local function listdir_recursive_copy(src, dst)
    local entries, err = sys.listdir(src)
    if not entries then
        return nil, err
    end

    local ok_mkdir, err_mkdir = sys.mkdir(dst, true)
    if not ok_mkdir then
        return nil, err_mkdir
    end

    for _, name in ipairs(entries) do
        local src_child = src .. DIR_SEP .. name
        local dst_child = dst .. DIR_SEP .. name
        local ok_copy, err_copy = sys.copy(src_child, dst_child)
        if not ok_copy then
            return nil, err_copy
        end
    end

    return true
end

local function remove_tree(path)
    if lfs.attributes(path, "mode") == "directory" then
        for name in lfs.dir(path) do
            if name ~= "." and name ~= ".." then
                local child = path .. DIR_SEP .. name
                local mode = lfs.attributes(child, "mode")
                if mode == "directory" then
                    local ok, err = remove_tree(child)
                    if not ok then
                        return nil, err
                    end
                else
                    local ok, err = os.remove(child)
                    if not ok then
                        return nil, err or ("failed to remove file: " .. child)
                    end
                end
            end
        end
        local ok, err = lfs.rmdir(path)
        if not ok then
            return nil, err or ("failed to remove directory: " .. path)
        end
        return true
    end

    local ok, err = os.remove(path)
    if ok then
        return true
    end
    return nil, err or ("failed to remove path: " .. path)
end

function sys.readfile(path)
    local p, err = normalize_path(path)
    if not p then
        return nil, err
    end

    local f, open_err = io.open(p, "rb")
    if not f then
        return nil, open_err
    end
    local data = f:read("*a")
    f:close()
    return data
end

function sys.writefile(path, data)
    local p, err = normalize_path(path)
    if not p then
        return nil, err
    end

    local parent = dirname(p)
    if parent and parent ~= "" then
        local ok_mkdir, mkdir_err = sys.mkdir(parent, true)
        if not ok_mkdir then
            return nil, mkdir_err
        end
    end

    local f, open_err = io.open(p, "wb")
    if not f then
        return nil, open_err
    end

    local ok_write, write_err = f:write(data or "")
    f:close()
    if not ok_write then
        return nil, write_err or ("failed to write file: " .. p)
    end
    return true
end

function sys.exists(path)
    local p = normalize_path(path)
    if not p then
        return false
    end
    return stat(p) ~= nil
end

function sys.isdir(path)
    local p, err = normalize_path(path)
    if not p then
        return nil, err
    end

    local info = stat(p)
    if not info then
        return false
    end
    return info.mode == "directory"
end

function sys.listdir(path)
    local p, err = normalize_path(path)
    if not p then
        return nil, err
    end

    if not sys.isdir(p) then
        return nil, "not a directory: " .. p
    end

    local entries = {}
    for name in lfs.dir(p) do
        if name ~= "." and name ~= ".." then
            entries[#entries + 1] = name
        end
    end

    table.sort(entries)
    return entries
end

function sys.copy(src, dst)
    local s, err_src = normalize_path(src)
    if not s then
        return nil, err_src
    end
    local d, err_dst = normalize_path(dst)
    if not d then
        return nil, err_dst
    end

    if not sys.exists(s) then
        return nil, "source does not exist: " .. s
    end

    if sys.isdir(s) then
        return listdir_recursive_copy(s, d)
    end

    local parent = dirname(d)
    if parent and parent ~= "" then
        local ok_mkdir, mkdir_err = sys.mkdir(parent, true)
        if not ok_mkdir then
            return nil, mkdir_err
        end
    end

    local in_f, in_err = io.open(s, "rb")
    if not in_f then
        return nil, in_err
    end
    local out_f, out_err = io.open(d, "wb")
    if not out_f then
        in_f:close()
        return nil, out_err
    end

    while true do
        local block = in_f:read(64 * 1024)
        if not block then
            break
        end
        local ok_write, write_err = out_f:write(block)
        if not ok_write then
            in_f:close()
            out_f:close()
            return nil, write_err or ("failed writing destination: " .. d)
        end
    end

    in_f:close()
    out_f:close()
    return true
end

function sys.move(src, dst)
    local s, err_src = normalize_path(src)
    if not s then
        return nil, err_src
    end
    local d, err_dst = normalize_path(dst)
    if not d then
        return nil, err_dst
    end

    local parent = dirname(d)
    if parent and parent ~= "" then
        local ok_mkdir, mkdir_err = sys.mkdir(parent, true)
        if not ok_mkdir then
            return nil, mkdir_err
        end
    end

    local ok, rename_err = os.rename(s, d)
    if ok then
        return true
    end

    local ok_copy, copy_err = sys.copy(s, d)
    if not ok_copy then
        return nil, rename_err or copy_err
    end

    local ok_remove, remove_err = remove_tree(s)
    if not ok_remove then
        return nil, remove_err
    end

    return true
end

function sys.mkdir(path, recursive)
    local p, err = normalize_path(path)
    if not p then
        return nil, err
    end

    if recursive == false then
        return mkdir_one(p)
    end

    if sys.exists(p) then
        if sys.isdir(p) then
            return true
        end
        return nil, "path exists but is not a directory: " .. p
    end

    local prefix = ""
    local rest = p
    if p:sub(1, 1) == "/" or p:sub(1, 1) == "\\" then
        prefix = p:sub(1, 1)
        rest = p:sub(2)
    elseif p:match("^[A-Za-z]:[/\\]") then
        prefix = p:sub(1, 3)
        rest = p:sub(4)
    end

    local current = prefix
    for segment in rest:gmatch("[^/\\]+") do
        if current == "" or current:sub(-1) == "/" or current:sub(-1) == "\\" then
            current = current .. segment
        else
            current = current .. DIR_SEP .. segment
        end

        local ok_mkdir, mkdir_err = mkdir_one(current)
        if not ok_mkdir then
            return nil, mkdir_err
        end
    end

    return true
end

function sys.cwd()
    local dir, err = lfs.currentdir()
    if dir then
        return dir
    end
    return nil, err
end

-- socket.sleep waits without starting a process, to the millisecond.
-- Windows has no socket.core, and waits for ping as turbolua did.
local has_socket, socket_core = pcall(require, "socket.core")

function sys.sleep(seconds)
    local n = tonumber(seconds)
    if not n or n < 0 then
        n = 1
    end

    if has_socket then
        socket_core.sleep(n)
        return true
    end

    local ticks = math.floor(n + 0.999)
    local ok, why, code = os.execute("ping -n " .. tostring(ticks + 1) .. " 127.0.0.1 >NUL")
    return command_ok(ok, why, code)
end

function sys.execute(command)
    if type(command) ~= "string" or command == "" then
        return nil, "command must be a non-empty string"
    end
    return os.execute(command)
end

function sys.input(prompt)
    if prompt ~= nil then
        io.write(tostring(prompt))
        io.flush()
    end
    return io.read("*l")
end

return sys

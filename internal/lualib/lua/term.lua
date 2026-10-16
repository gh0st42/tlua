-- ANSI terminal helper library (Turbo Pascal/QBasic style)

local term = {}

local ESC = "\27"
term.ESC = ESC

-- 16-color EGA palette indices (QBasic/Turbo Pascal order).
term.color = {
    black = 0,
    blue = 1,
    green = 2,
    cyan = 3,
    red = 4,
    magenta = 5,
    brown = 6,
    lightgray = 7,
    darkgray = 8,
    lightblue = 9,
    lightgreen = 10,
    lightcyan = 11,
    lightred = 12,
    lightmagenta = 13,
    yellow = 14,
    white = 15,
    gray = 8,
    grey = 8,
    lightgrey = 7,
}

local ega_to_ansi = {
    [0] = 0,
    [1] = 4,
    [2] = 2,
    [3] = 6,
    [4] = 1,
    [5] = 5,
    [6] = 3,
    [7] = 7,
}

local state = {
    fg = 7,
    bg = 0,
}

local tty_state = {
    raw = false,
    saved = nil,
}

local function write_sequence(sequence, out)
    local stream = out or io.stdout
    stream:write(sequence)
    return stream
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

local function run_capture(command)
    local pipe = io.popen(command, "r")
    if not pipe then
        return nil
    end
    local data = pipe:read("*a")
    pipe:close()
    if not data then
        return nil
    end
    data = data:gsub("%s+$", "")
    if data == "" then
        return nil
    end
    return data
end

local function clamp_integer(value, fallback)
    local n = tonumber(value)
    if not n then
        return fallback
    end
    return math.floor(n)
end

local function clamp_positive(value, fallback)
    local n = clamp_integer(value, fallback)
    if n < 1 then
        return fallback
    end
    return n
end

local function resolve_ega_color(value)
    if type(value) == "string" then
        return term.color[value:lower()]
    end

    local n = clamp_integer(value, -1)
    if n < 0 or n > 15 then
        return nil
    end
    return n
end

local function fg_sgr(index)
    local bright = index >= 8
    local base = index % 8
    local ansi_index = ega_to_ansi[base]
    return (bright and 90 or 30) + ansi_index
end

local function bg_sgr(index)
    local bright = index >= 8
    local base = index % 8
    local ansi_index = ega_to_ansi[base]
    return (bright and 100 or 40) + ansi_index
end

local function csi(code, out)
    return write_sequence(ESC .. "[" .. tostring(code), out)
end

local function sgr(codes, out)
    if type(codes) == "table" then
        return csi(table.concat(codes, ";") .. "m", out)
    end
    return csi(tostring(codes) .. "m", out)
end

function term.raw(enabled)
    if type(enabled) ~= "boolean" then
        return nil, "raw mode expects true or false"
    end
    if package.config:sub(1, 1) == "\\" then
        return nil, "raw mode is not supported on Windows"
    end

    if enabled then
        if not tty_state.saved then
            tty_state.saved = run_capture("stty -g 2>/dev/null")
        end
        -- Use key-read mode without disabling output post-processing.
        -- -ixon stops the tty from swallowing Ctrl+Q and Ctrl+S as flow control.
        local ok, why, code = os.execute("stty -icanon -echo -ixon min 1 time 0 2>/dev/null")
        if not command_ok(ok, why, code) then
            return nil, "failed to enable raw mode"
        end
        tty_state.raw = true
        return true
    end

    local restore_cmd = "stty sane 2>/dev/null"
    if tty_state.saved and tty_state.saved ~= "" then
        restore_cmd = "stty " .. tty_state.saved .. " 2>/dev/null"
    end
    local ok, why, code = os.execute(restore_cmd)
    if not command_ok(ok, why, code) then
        return nil, "failed to disable raw mode"
    end
    tty_state.raw = false
    return true
end

function term.size()
    local cols = clamp_positive(os.getenv("COLUMNS"), 0)
    local rows = clamp_positive(os.getenv("LINES"), 0)
    if cols > 0 and rows > 0 then
        return cols, rows
    end

    local data = run_capture("stty size 2>/dev/null")
    if not data then
        return nil, "unable to detect terminal size"
    end

    local r, c = data:match("^(%d+)%s+(%d+)$")
    rows = clamp_positive(r, 0)
    cols = clamp_positive(c, 0)
    if rows < 1 or cols < 1 then
        return nil, "unable to parse terminal size"
    end

    return cols, rows
end

function term.readkey(timeout)
    if timeout == nil then
        return io.read(1)
    end
    if package.config:sub(1, 1) == "\\" then
        return io.read(1)
    end
    local tenths = clamp_integer((tonumber(timeout) or 0) * 10, 0)
    if tenths < 0 then tenths = 0 end
    if tenths > 255 then tenths = 255 end
    os.execute(("stty min 0 time %d 2>/dev/null"):format(tenths))
    local ch = io.read(1)
    os.execute("stty min 1 time 0 2>/dev/null")
    return ch
end

-- The same as sys.sleep.
function term.sleep(seconds)
    return require("tlua.sys").sleep(seconds)
end

function term.cursor(visible, out)
    if type(visible) ~= "boolean" then
        return nil, "cursor expects true or false"
    end
    if visible then
        return csi("?25h", out)
    end
    return csi("?25l", out)
end

function term.clrscr(color, out)
    if color ~= nil then
        local ok, err = term.textbackground(color, out)
        if not ok then
            return nil, err
        end
    end
    csi("2J", out)
    return csi("H", out)
end

function term.gotoxy(x, y, out)
    local col = clamp_positive(x, 1)
    local row = clamp_positive(y, 1)
    return csi(row .. ";" .. col .. "H", out)
end

function term.textcolor(color, out)
    local idx = resolve_ega_color(color)
    if idx == nil then
        return nil, "text color must be in [0, 15] or a known color name"
    end
    state.fg = idx
    return sgr(fg_sgr(idx), out)
end

function term.textbackground(color, out)
    local idx = resolve_ega_color(color)
    if idx == nil then
        return nil, "background color must be in [0, 15] or a known color name"
    end
    state.bg = idx
    return sgr(bg_sgr(idx), out)
end

function term.colorize(fg, bg, out)
    local fg_idx = resolve_ega_color(fg)
    if fg_idx == nil then
        return nil, "foreground color must be in [0, 15] or a known color name"
    end
    local bg_idx = resolve_ega_color(bg)
    if bg_idx == nil then
        return nil, "background color must be in [0, 15] or a known color name"
    end
    state.fg = fg_idx
    state.bg = bg_idx
    return sgr({ fg_sgr(fg_idx), bg_sgr(bg_idx) }, out)
end

function term.normvideo(out)
    state.fg = 7
    state.bg = 0
    return sgr({ 0, fg_sgr(state.fg), bg_sgr(state.bg) }, out)
end

function term.highvideo(out)
    return sgr(1, out)
end

function term.lowvideo(out)
    return sgr(2, out)
end

function term.flush(out)
    local stream = out or io.stdout
    return stream:flush()
end

local function tag_color_value(token)
    if token:match("^%d+$") then
        return tonumber(token)
    end
    return token
end

local function apply_tag(tag, out)
    local lower = tag:lower()
    if lower == "reset" or lower == "normal" then
        return term.normvideo(out)
    elseif lower == "high" or lower == "bright" then
        return term.highvideo(out)
    elseif lower == "low" or lower == "dim" then
        return term.lowvideo(out)
    end

    local bg_only = tag:match("^[bB][gG]:(.+)$")
    if bg_only then
        return term.textbackground(tag_color_value(bg_only), out)
    end

    local fg_only = tag:match("^[fF][gG]:(.+)$")
    if fg_only then
        return term.textcolor(tag_color_value(fg_only), out)
    end

    local fg_part, bg_part = tag:match("^([^,]+),([^,]+)$")
    if fg_part and bg_part then
        return term.colorize(tag_color_value(fg_part), tag_color_value(bg_part), out)
    end

    return term.textcolor(tag_color_value(tag), out)
end

local function write_colored(str, out, newline)
    local stream = out or io.stdout
    local len = #str
    local buffer = {}
    local used_tag = false
    local i = 1

    local function flush_buffer()
        if #buffer > 0 then
            stream:write(table.concat(buffer))
            buffer = {}
        end
    end

    while i <= len do
        local c = str:sub(i, i)
        if c == "{" and str:sub(i + 1, i + 1) == "{" then
            buffer[#buffer + 1] = "{"
            i = i + 2
        elseif c == "}" and str:sub(i + 1, i + 1) == "}" then
            buffer[#buffer + 1] = "}"
            i = i + 2
        elseif c == "{" then
            local close = str:find("}", i + 1, true)
            if not close then
                buffer[#buffer + 1] = c
                i = i + 1
            else
                local tag = str:sub(i + 1, close - 1)
                flush_buffer()
                local ok = apply_tag(tag, stream)
                if not ok then
                    buffer[#buffer + 1] = "{" .. tag .. "}"
                else
                    used_tag = true
                end
                i = close + 1
            end
        else
            buffer[#buffer + 1] = c
            i = i + 1
        end
    end

    flush_buffer()

    if newline then
        stream:write("\n")
    end

    if used_tag then
        term.normvideo(stream)
    end

    return stream
end

function term.print(str, out)
    return write_colored(tostring(str), out, false)
end

function term.println(str, out)
    return write_colored(tostring(str), out, true)
end

return term

-- sys walkthrough: tlua examples/console/sys_demo.lua

local sys = require("sys")

local sep = package.config:sub(1, 1)
-- A folder beside a temporary file's name, so nothing is left where the
-- demo was started.
local tmp = os.tmpname()
os.remove(tmp)
local demo_dir = tmp .. "-sys-demo"
local notes_dir = demo_dir .. sep .. "notes"
local hello_file = notes_dir .. sep .. "hello.txt"
local copy_file = demo_dir .. sep .. "hello_copy.txt"
local moved_file = demo_dir .. sep .. "hello_moved.txt"

io.write("sys walkthrough\n")
io.write("This script creates files in ", demo_dir, " so you can inspect results.\n")

io.write("\n== Current Directory ==\n")
local cwd, cwd_err = sys.cwd()
if cwd then
    io.write("sys.cwd(): ", tostring(cwd), "\n")
else
    io.write("sys.cwd() failed: ", tostring(cwd_err), "\n")
end

io.write("\n== Create Directories ==\n")
local ok_mkdir, err_mkdir = sys.mkdir(notes_dir, true)
io.write("sys.mkdir(notes_dir, true): ", tostring(ok_mkdir), "\n")
if err_mkdir then
    io.write("sys.mkdir failed: ", tostring(err_mkdir), "\n")
end

io.write("\n== Write And Read Files ==\n")
local ok_write, err_write = sys.writefile(hello_file, "hello from sys.writefile()\n")
io.write("sys.writefile(hello.txt): ", tostring(ok_write), "\n")
if err_write then
    io.write("sys.writefile failed: ", tostring(err_write), "\n")
end

local text, err_read = sys.readfile(hello_file)
if text then
    io.write("sys.readfile(hello.txt): ", (text:gsub("\n$", "")), "\n")
else
    io.write("sys.readfile failed: ", tostring(err_read), "\n")
end

io.write("\n== Check Paths ==\n")
io.write("sys.exists(hello.txt): ", tostring(sys.exists(hello_file)), "\n")
io.write("sys.isdir(notes_dir): ", tostring(sys.isdir(notes_dir)), "\n")

io.write("\n== Copy And Move ==\n")
local ok_copy, err_copy = sys.copy(hello_file, copy_file)
io.write("sys.copy(hello.txt -> hello_copy.txt): ", tostring(ok_copy), "\n")
if err_copy then
    io.write("sys.copy failed: ", tostring(err_copy), "\n")
end

local ok_move, err_move = sys.move(copy_file, moved_file)
io.write("sys.move(hello_copy.txt -> hello_moved.txt): ", tostring(ok_move), "\n")
if err_move then
    io.write("sys.move failed: ", tostring(err_move), "\n")
end

io.write("\n== List Directory ==\n")
local entries, err_list = sys.listdir(demo_dir)
if entries then
    io.write("sys.listdir(demo_dir) items: ", tostring(#entries), "\n")
    for i = 1, #entries do
        io.write("- ", entries[i], "\n")
    end
else
    io.write("sys.listdir failed: ", tostring(err_list), "\n")
end

io.write("\n== Execute Command ==\n")
io.write("Running sys.execute to print one line from the shell:\n")
if sep == "\\" then
    sys.execute("echo hello from sys.execute()")
else
    sys.execute("printf 'hello from sys.execute()\\n'")
end

io.write("\n== Sleep ==\n")
io.write("Pausing for 0.3 seconds using sys.sleep(0.3)...\n")
sys.sleep(0.3)
io.write("Done.\n")

io.write("\n== Input ==\n")
local name = sys.input("What is your name? ")
io.write("sys.input() returned: ", tostring(name), "\n")

io.write("\nWalkthrough complete. You can now inspect files under ", demo_dir, ".\n")

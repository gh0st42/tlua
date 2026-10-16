# `argparse` — command-line argument parsing

[argparse](https://github.com/luarocks/argparse) is a feature-rich
command-line argument parser modeled after Python's `argparse`. It
supports positional arguments, options/flags, subcommands, mutually
exclusive groups, type conversion, choices, and auto-generated
`--help`/usage text and shell completion scripts.

```lua
local argparse = require("argparse")
```

## Creating a parser

`argparse(name, description, epilog)` creates and returns a new parser
(a `-h`/`--help` flag is added automatically).

- `name` (string, optional) — program name shown in usage text; defaults
  to `arg[0]`.
- `description` (string, optional) — shown before the option list in
  `--help` output.
- `epilog` (string, optional) — shown after the option list.

```lua
local argparse = require("argparse")

local parser = argparse("greet", "Greets someone by name.")
```

## Defining arguments and options

Every element (`argument`, `option`, `flag`, `command`) is configured
either by chaining setter methods, or by "calling" it with positional
values (in a fixed order per element type) or a table of named
properties. All three styles below are equivalent:

```lua
parser:option("-o --output"):description("Output file"):default("out.txt")
parser:option("-o --output", "Output file", "out.txt")
parser:option("-o --output") { description = "Output file", default = "out.txt" }
```

### `parser:argument(name, description, default, convert, args)`

Adds a required positional argument.

- `name` (string) — the argument's name, used in usage text and as the
  default result key.
- `description` (string, optional)
- `default` (optional) — value used when the argument is omitted (only
  possible when `args` allows zero occurrences, e.g. `"?"`).
- `convert` (function or table, optional) — see
  [Type conversion](#type-conversion).
- `args` (number or string, optional) — how many values this argument
  consumes; see [Argument/option counts](#argumentoption-counts).
- Returns: the `Argument` object (for further chaining).

```lua
parser:argument("input", "File to process")
local args = parser:parse({ "data.csv" })
print(args.input) --> data.csv
```

### `parser:option(name, description, default, convert, args, count)`

Adds a named option that takes one or more values, e.g. `-o file` or
`--output=file`.

- `name` (string) — space-separated aliases, e.g. `"-o --output"`.
  Underscores in an alias also accept hyphens (`--dry_run` also matches
  `--dry-run`).
- `description` (string, optional)
- `default` (optional) — value used when the option is never given.
- `convert` (function or table, optional)
- `args` (number or string, optional) — values consumed per invocation
  (default `1`).
- `count` (number or string, optional) — how many times the option itself
  may be given (default `0-1`; use `"*"` to allow repeats, combined with
  `action = "append"` or `"concat"` to collect all values).
- Returns: the `Option` object.

```lua
parser:option("-o --output", "Output file", "out.txt")
local args = parser:parse({ "--output=result.txt" })
print(args.output) --> result.txt
```

### `parser:flag(name, description, default)`

A convenience wrapper around `option` with `args` fixed at `0` — a
boolean switch. Its default action is `store_true` (or, if you allow it
to be given more than once via `:count("*")`, it automatically switches
to counting invocations instead — see [Actions](#actions)).

```lua
parser:flag("-v --verbose", "Enable verbose output")
local args = parser:parse({ "-v" })
print(args.verbose) --> true
```

### Where results are stored

Unless overridden with `:target("name")`, a value is stored under a key
derived from the element's name: for an argument, its name as given; for
an option, its long (`--`) alias if it has one (otherwise its short
alias), with the leading dashes stripped and any remaining dashes turned
into underscores. So `parser:option("-o --output")` and
`parser:argument("input")` populate `args.output` and `args.input` in the
table returned by [`parser:parse()`](#parsing-arguments).

## Argument/option counts

The `args` (and option `count`) properties accept:

| Value      | Meaning                                   |
|------------|--------------------------------------------|
| a number `N` | exactly `N` values/invocations           |
| `"?"`      | zero or one                                |
| `"*"`      | zero or more                               |
| `"+"`      | one or more                                |
| `"N-M"`    | between `N` and `M`, inclusive             |
| `"N+"`     | `N` or more                                |

```lua
parser:option("--point", "x y coordinates"):args(2)
parser:argument("files"):args("+") -- one or more positional args, collected into a list
```

## Type conversion

`convert` transforms the raw string argument into another value, and can
be:

- a **function** `function(str) -> value[, errmsg]` — return `nil, "some message"`
  to reject the value with a custom error;
- a **table** used as a lookup map, `convert = { yes = true, no = false }`
  — the raw string is looked up in the table, and an unknown value is
  rejected automatically.

```lua
parser:option("-n --count", "How many times"):convert(tonumber)
parser:option("--mode"):convert({ fast = 1, slow = 2 })
```

### `choices`

Restricts accepted values to a fixed list (checked before conversion):

```lua
parser:option("--level"):choices({ "low", "medium", "high" })
```

## Actions

Every element has an `action` that decides how its parsed value(s) are
written into the result table. The default action depends on the
element's `count` (options only — arguments are always `count` `1`),
i.e. the maximum number of times it may be given:

| Situation                              | Default action | Result                          |
|-----------------------------------------|----------------|----------------------------------|
| takes no values, `count` max is `1` (the default) | `"store_true"` | `true`                 |
| takes values, `count` max is `1`        | `"store"`      | the value (or values, if `args` allows more than one) |
| takes no values, `count` max is more than `1` | `"count"`  | number of times it was given     |
| takes values, `count` max is more than `1` | `"append"`  | a list of values, one per invocation |

By default (`count` max `1`), giving an option more than once doesn't
raise an error — it simply keeps the last value (`overwrite` is `true`
by default). Set `count` to something like `"*"` or `"0-3"` to switch to
the repeatable behavior above, or call `:overwrite(false)` to make any
invocation past `count`'s maximum a parse error instead.

```lua
-- Counts invocations automatically (default "count" action kicks in):
parser:flag("-v --verbose"):count("*")
local args = parser:parse({ "-v", "-v", "-v" })
print(args.verbose) --> 3

-- Collects one value per invocation (default "append" action kicks in):
parser:option("-D --define"):count("*")
local args2 = parser:parse({ "-D", "a=1", "-D", "b=2" })
print(args2.define[1], args2.define[2]) --> a=1  b=2
```

You can also explicitly request `"concat"` (append every value from a
single multi-value invocation into one flat list — mainly relevant for
`args("+")`/`args("*")` combined with a `count` greater than one), or
pass your own `function(result, target, value, overwrite)` for full
control over how a value is recorded.

## Commands (subcommands)

`parser:command(name, description, epilog)` adds a subcommand, itself a
full parser (it can have its own arguments, options, and even nested
subcommands). A handful of help/usage formatting properties (like margin
and width settings) fall back to the parent parser's if not set on the
command, but things like `description`, `epilog`, and `action` are not
inherited and must be set on the command itself if wanted.

```lua
local parser = argparse("git-like", "A tool with subcommands")

local add = parser:command("add", "Add file contents to the index")
add:argument("files"):args("+")

local commit = parser:command("commit", "Record changes")
commit:option("-m --message", "Commit message")

local args = parser:parse({ "commit", "-m", "Initial commit" })
print(args.commit)  --> true  (the invoked command's own flag)
print(args.message) --> Initial commit
```

Use `parser:command_target("name")` on the top-level parser if you want
the invoked command's name recorded under a key, which is convenient for
dispatching:

```lua
parser:command_target("command")

local args = parser:parse({ "add", "file.txt" })
if args.command == "add" then
    -- handle add
end
```

A command (or the top-level parser itself) can also carry its own
`action` callback, `function(result, name)`. It runs automatically as the
last step of a successful `parser:parse()`/`pparse()` call, after all
arguments and options have been validated — innermost command first:

```lua
add:action(function(result, name)
    print("adding", table.concat(result.files, ", "))
end)
```

## Mutually exclusive options: `parser:mutex(...)`

Declares that only one of the given options/arguments may be used at a
time; using more than one is a parse error.

```lua
local quiet = parser:flag("-q --quiet")
local verbose = parser:flag("-v --verbose")
parser:mutex(quiet, verbose)
```

## Grouping options in `--help`: `parser:group(name, ...)`

Purely cosmetic — groups the given elements under a labeled section in
generated help text.

```lua
parser:group("Output options",
    parser:option("-o --output"),
    parser:flag("-v --verbose")
)
```

## Parsing arguments

### `parser:parse(args)`

Parses `args` (defaults to the process's `arg` table) and returns the
result table. On a parse error, it prints usage + the error message to
stderr and calls `os.exit(1)` — suitable for a top-level script.

```lua
local args = parser:parse() -- parses arg from the command line
```

### `parser:pparse(args)`

Like `parse`, but never exits the process: returns `true, result` on
success or `false, errmsg` on failure, so you can handle errors yourself
(e.g. when embedding a parser in a larger application).

```lua
local ok, result = parser:pparse({ "--bad-flag" })
if not ok then
    print("error: " .. result)
    return
end
```

### `parser:error(msg)`

Prints usage and `msg` to stderr and exits with status `1` — the same
routine used internally for parse errors; call it yourself for
validation failures that argparse itself can't detect (e.g. cross-field
constraints checked after parsing).

## Help text and shell completion

- `parser:get_usage()` / `parser:get_help()` — return the usage line /
  full help text as strings (used internally by the auto `-h/--help`
  flag, and printed via `print(parser:get_help())`).
- `parser:add_help(boolean_or_name)` — enable/disable/rename the
  automatic help option (enabled by default).
- `parser:add_complete()` / `parser:add_complete_command()` — add a
  `--completion` option or `completion` command that prints a shell
  completion script; `parser:get_bash_complete()`,
  `parser:get_zsh_complete()`, `parser:get_fish_complete()` return the
  script text directly for bash, zsh, and fish respectively.

## Example: a small CLI with global flags and subcommands

```lua
local argparse = require("argparse")

local parser = argparse("mytool", "Example CLI with subcommands")
parser:flag("-v --verbose", "Enable verbose output")
parser:command_target("command")

local build = parser:command("build", "Build the project")
build:option("-o --output", "Output directory", "dist")
build:flag("--release", "Build in release mode")

local clean = parser:command("clean", "Remove build artifacts")

local ok, args = parser:pparse()
if not ok then
    print(args) -- args holds the error message here
    os.exit(1)
end

if args.verbose then
    print("verbose mode on")
end

if args.command == "build" then
    print(("building into %s (release=%s)"):format(args.output, tostring(args.release)))
elseif args.command == "clean" then
    print("cleaning...")
end
```

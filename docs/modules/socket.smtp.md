# `socket.smtp` — SMTP client

`socket.smtp` sends email over SMTP, built on
[`socket.tp`](socket.tp.md). `smtp.message` builds a properly-formatted
(and dot-stuffed) MIME message — including multipart messages — as an
[LTN12 source](socket.md#sources-and-sinks-ltn12-helpers), and
`smtp.send` delivers it.

```lua
local smtp = require("socket.smtp")
```

## `smtp.send(mailt)`

Connects, authenticates (if credentials are given), and sends a message.

| Field      | Meaning                                                     |
|------------|-----------------------------------------------------------------|
| `from`     | envelope sender, **in angle brackets**: `"<sender@example.com>"` |
| `rcpt`     | envelope recipient(s): a single angle-bracketed address, or an array of them |
| `source`   | an LTN12 source producing the full message (headers + body) — normally the result of [`smtp.message`](#smtpmessagemesgt) |
| `server`, `port` | SMTP server to connect to (default `smtp.SERVER`/`smtp.PORT`) |
| `domain`   | domain name to announce in `EHLO` (default `smtp.DOMAIN`)        |
| `user`, `password` | credentials, if the server requires authentication (LOGIN or PLAIN, negotiated automatically) |
| `create`   | a socket constructor to use instead of `socket.tcp`               |
| `step`     | an LTN12 pump step function (advanced)                            |

Returns `1` on success, or `nil, errmsg` on failure.

> Note the distinction: `from`/`rcpt` are the raw SMTP envelope addresses
> (bare, angle-bracketed), separate from the human-readable `From:`/`To:`
> headers you put in the message itself via `smtp.message`.

```lua
local smtp = require("socket.smtp")

local ok, err = smtp.send({
    from = "<sender@example.com>",
    rcpt = "<receiver@example.com>",
    source = smtp.message({
        headers = {
            from = "Sender <sender@example.com>",
            to = "Receiver <receiver@example.com>",
            subject = "Hello from tlua",
        },
        body = "This is the message body.\r\n",
    }),
    server = "smtp.example.com",
    port = 587,
    user = "sender@example.com",
    password = "s3cret",
})
if not ok then
    error(err)
end
```

Sending to multiple recipients:

```lua
smtp.send({
    from = "<sender@example.com>",
    rcpt = { "<a@example.com>", "<b@example.com>" },
    source = smtp.message({ headers = { subject = "Hi" }, body = "Hi both!\r\n" }),
})
```

## `smtp.message(mesgt)`

Builds a MIME message as an LTN12 source, handling header formatting and
[dot-stuffing](https://en.wikipedia.org/wiki/SMTP#Extensions) (which
`smtp.send`/`data` apply automatically — you don't need to do it
yourself).

- `mesgt.headers` (table) — message headers, e.g. `from`, `to`,
  `subject`, `cc` (keys are case-insensitive; well-known names are
  capitalized correctly, e.g. `mime-version`/`x-mailer`/`date` are
  filled in automatically if not given).
- `mesgt.body` — the message body, one of:
  - a **string** — sent as-is (a default `Content-Type:
    text/plain; charset="iso-8859-1"` is added if you didn't set one);
  - a **function** (an LTN12 source) — pumped chunk by chunk, useful for
    streaming large or encoded bodies;
  - a **table** — a multipart message: an array of sub-message tables
    (each itself shaped like `mesgt`, with its own `headers`/`body`),
    plus optional `preamble`/`epilogue` string fields. A
    `multipart/mixed` (or your own) `Content-Type` with a generated
    boundary is added automatically.
- Returns: a function usable as an LTN12 source (i.e. as `mailt.source`).

### Plain text message

```lua
local smtp = require("socket.smtp")

local source = smtp.message({
    headers = { subject = "Plain text" },
    body = "Just a plain message.\r\n",
})
```

### Multipart message with an attachment

```lua
local smtp = require("socket.smtp")
local ltn12 = require("ltn12")
local mime = require("mime")

local source = smtp.message({
    headers = { subject = "With attachment" },
    body = {
        preamble = "This is a multipart MIME message.",
        [1] = {
            body = "See the attached file.\r\n",
        },
        [2] = {
            headers = {
                ["content-type"] = "application/octet-stream",
                ["content-disposition"] = 'attachment; filename="data.bin"',
                ["content-transfer-encoding"] = "BASE64",
            },
            body = ltn12.source.chain(
                ltn12.source.file(io.open("data.bin", "rb")),
                mime.encode("base64")
            ),
        },
    },
})

smtp.send({ from = "<me@example.com>", rcpt = "<you@example.com>", source = source })
```

## Defaults

- `smtp.SERVER` — default server (`"localhost"`).
- `smtp.PORT` — default port (`25`).
- `smtp.DOMAIN` — domain announced in `EHLO` (defaults to the
  `SERVER_NAME` environment variable, or `"localhost"`).
- `smtp.ZONE` — default timezone offset used in the generated `Date:`
  header when none is given (`"-0000"`, meaning "unknown").
- `smtp.TIMEOUT` — connection timeout in seconds (`60`).

## Low-level API

`smtp.open(server, port, create)` returns a connection object for manual
control — the pieces `smtp.send` composes:

- `s:greet(domain)` — sends `EHLO`; returns the server's extension list
  (used to pick an auth method).
- `s:auth(user, password, ext)` — authenticates using whichever of
  `AUTH LOGIN`/`AUTH PLAIN` the server advertised in `ext`; a no-op if
  `user`/`password` are `nil`.
- `s:mail(from)` / `s:rcpt(to)` — envelope sender/recipient.
- `s:data(source, step)` — sends the message body from an LTN12 source
  (dot-stuffing is applied here).
- `s:send(mailt)` — convenience combining `mail`+`rcpt`(+ each
  recipient)+`data` from a `mailt` table.
- `s:quit()` / `s:close()`.

```lua
local smtp = require("socket.smtp")

local s = assert(smtp.open("smtp.example.com", 25))
local ext = s:greet()
s:mail("<me@example.com>")
s:rcpt("<you@example.com>")
s:data(smtp.message({ headers = { subject = "Hi" }, body = "Hi!\r\n" }))
s:quit()
s:close()
```

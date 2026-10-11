---@meta ssl
--- TLS, `require "ssl"`: LuaSec 1.3.2, on Go's crypto/tls. A LuaSocket TCP
--- connection is wrapped in TLS, as a client or a server:
---
---     local conn = assert(ssl.wrap(tcp, { mode = "client", protocol = "any" }))
---     conn:sni("example.org")
---     assert(conn:dohandshake())
---
--- `require "ssl.https"` fetches https:// addresses, as socket.http does.
--- Linux and macOS.

local ssl = {}

ssl._VERSION = "1.3.2"

--- What newcontext and wrap take.
---@class ssl.Params
---@field mode "client"|"server"
---@field protocol? "any"|"tlsv1"|"tlsv1_1"|"tlsv1_2"|"tlsv1_3"
---@field key? string a PEM file's path, or the PEM itself
---@field certificate? string the certificate (and its chain) for key, likewise
---@field certificates? { key: string, certificate: string, password?: string|fun(): string }[]
---@field password? string|fun(): string for an encrypted key
---@field cafile? string the certificates to trust; the system's when neither this nor capath is given
---@field capath? string a folder of them
---@field verify? string|string[] "none" (the default: nothing is checked), "peer", "fail_if_no_peer_cert"
---@field verifyext? string|string[] "lsec_continue" (go on whatever the check finds), "lsec_ignore_purpose"
---@field options? string|string[] "all", "no_tlsv1_2", "no_tlsv1_3", ...
---@field alpn? string|string[]|fun(offered: string[]): string|string[] protocols to offer, or a server's choice
---@field curve? string "prime256v1", "secp384r1", "secp521r1", "X25519"
---@field curveslist? string the same, joined by ":"
---@field depth? integer the longest chain to accept
---@field ciphers? string accepted; Go chooses the cipher suites

--- A context made of params, for wrap to use again and again.
---@param params ssl.Params
---@return userdata? context
---@return string? why
function ssl.newcontext(params) end

--- Wraps a connected TCP socket in TLS. The socket is the connection's from
--- then on; use the connection instead.
---@param sock table a LuaSocket TCP socket
---@param params ssl.Params|userdata params, or a context newcontext made
---@return ssl.Connection? conn
---@return string? why
function ssl.wrap(sock, params) end

--- A certificate from its PEM.
---@param pem string
---@return ssl.Certificate?
function ssl.loadcertificate(pem) end

--- What this build has: options, protocols, curves, capabilities.
ssl.config = {}

---@class ssl.Connection
local Connection = {}

--- Shakes hands. With a timeout of 0 (or one that runs out) it says
--- false, "wantread" until the handshake is done: call it again.
---@return boolean ok
---@return string? why "wantread", "certificate verify failed", "closed", ...
function Connection:dohandshake() end

--- Sends data, or part of it (i to j), as a LuaSocket socket does.
---@param data string
---@param i? integer
---@param j? integer
---@return integer? last the last byte sent
---@return string? why "wantwrite" when it ran out of time, "closed", ...
---@return integer? partial
function Connection:send(data, i, j) end

--- Receives a line ("*l", the default), everything ("*a") or n bytes, as a
--- LuaSocket socket does.
---@param pattern? "*l"|"*a"|integer
---@param prefix? string
---@return string? data
---@return string? why "wantread" when it ran out of time, "closed", ...
---@return string? partial
function Connection:receive(pattern, prefix) end

---@param seconds? number nil or negative to wait as long as it takes
---@param mode? "b"|"t"
function Connection:settimeout(seconds, mode) end

function Connection:close() end

--- A client: the server name to ask for. A server: a context for each name
--- clients may ask for, and whether to refuse the others.
---@param name string|table<string, userdata>
---@param strict? boolean
function Connection:sni(name, strict) end

--- The peer's certificate (n = 1), or one further up its chain.
---@param n? integer
---@return ssl.Certificate?
function Connection:getpeercertificate(n) end

---@return ssl.Certificate[]
function Connection:getpeerchain() end

---@param n? integer
---@return ssl.Certificate?
function Connection:getlocalcertificate(n) end

---@return ssl.Certificate[]
function Connection:getlocalchain() end

--- Whether the peer's chain checked out against the trusted certificates,
--- and if not, why, in OpenSSL's words ("self-signed certificate", ...).
---@return boolean ok
---@return string? why
function Connection:getpeerverification() end

--- The protocol ALPN settled on.
---@return string?
function Connection:getalpn() end

--- The server name the client asked for.
---@return string?
function Connection:getsniname() end

--- The cipher, protocol ("TLSv1.3"), bits and the like, or one of them.
---@param field? string
---@return table|string|number?
function Connection:info(field) end

---@param label string
---@param length integer
---@param context? string
---@return string?
function Connection:exportkeyingmaterial(label, length, context) end

--- Whether there is something to read without waiting.
---@return boolean
function Connection:dirty() end

---@return integer
function Connection:getfd() end

--- "nothing", "read" or "write": what it last waited for.
---@return string
function Connection:want() end

---@return number received
---@return number sent
---@return number age
function Connection:getstats() end

---@class ssl.Certificate
local Certificate = {}

--- The fingerprint, in hexadecimal: "sha1" (the default), "sha256", "sha512".
---@param algorithm? "sha1"|"sha256"|"sha512"
---@return string
function Certificate:digest(algorithm) end

--- The subject's name, attribute by attribute: { oid =, name = "CN", value = }.
---@return { oid: string, name: string, value: string }[]
function Certificate:subject() end

---@return { oid: string, name: string, value: string }[]
function Certificate:issuer() end

--- The alternative names, under "2.5.29.17": dNSName, iPAddress,
--- rfc822Name and uniformResourceIdentifier lists.
---@return table
function Certificate:extensions() end

--- "Jan  2 15:04:05 2026 GMT"
---@return string
function Certificate:notbefore() end

---@return string
function Certificate:notafter() end

--- Whether it is valid at a time, as os.time() counts.
---@param time integer
---@return boolean
function Certificate:validat(time) end

--- The serial number in hexadecimal.
---@return string
function Certificate:serial() end

--- The public key as PEM, its kind ("RSA", "EC", ...) and its size in bits.
---@return string pem
---@return string kind
---@return integer bits
function Certificate:pubkey() end

---@return string
function Certificate:pem() end

---@return string?
function Certificate:getsignaturename() end

--- Whether this certificate issued subject, through any intermediates given.
---@param subject ssl.Certificate
---@param ... ssl.Certificate
---@return boolean? ok
---@return string? why
function Certificate:issued(subject, ...) end

return ssl

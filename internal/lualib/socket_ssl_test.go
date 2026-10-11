//go:build linux || darwin

package lualib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testPKI writes a CA, a server certificate it signed for localhost and
// 127.0.0.1, a self-signed one, and a client's, with their keys, into dir.
func testPKI(t *testing.T, dir string) {
	t.Helper()
	key := func(name string) *ecdsa.PrivateKey {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		der, _ := x509.MarshalECPrivateKey(k)
		os.WriteFile(filepath.Join(dir, name+".key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600)
		return k
	}
	write := func(name string, der []byte) *x509.Certificate {
		os.WriteFile(filepath.Join(dir, name+".pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
		c, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	now := time.Now()
	caKey := key("ca")
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "tlua test CA", Organization: []string{"tlua"}},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	ca := write("ca", caDER)

	srvKey := key("server")
	srvDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(0xbeef), Subject: pkix.Name{CommonName: "localhost", Country: []string{"DE"}},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		DNSNames: []string{"localhost", "example.test"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature,
	}, ca, &srvKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	write("server", srvDER)

	selfKey := key("self")
	selfTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "capsule.test"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		DNSNames: []string{"capsule.test"},
	}
	selfDER, _ := x509.CreateCertificate(rand.Reader, selfTmpl, selfTmpl, &selfKey.PublicKey, selfKey)
	write("self", selfDER)

	clientKey := key("client")
	clientTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(9), Subject: pkix.Name{CommonName: "a reader"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
	}
	clientDER, _ := x509.CreateCertificate(rand.Reader, clientTmpl, clientTmpl, &clientKey.PublicKey, clientKey)
	write("client", clientDER)
}

// sslHelpers set up a client and a server over a loopback connection and
// shake hands, the two in turn, without blocking: one Lua state plays both.
const sslHelpers = `
local socket = require "socket"
local ssl = require "ssl"
local function eq(got, want, what)
  if got ~= want then error(("%s: got %s, want %s"):format(what or "value", tostring(got), tostring(want)), 2) end
end
local function pair()
  local server = assert(socket.bind("127.0.0.1", 0))
  local _, port = server:getsockname()
  local c = assert(socket.connect("127.0.0.1", port))
  local s = assert(server:accept())
  server:close()
  return c, s
end
-- handshake runs both handshakes to their ends, and returns each one's
-- result and error.
local function handshake(c, s)
  c:settimeout(0); s:settimeout(0)
  local cdone, sdone, cok, cerr, sok, serr
  for _ = 1, 2000 do
    if not cdone then
      cok, cerr = c:dohandshake()
      cdone = cok or cerr ~= "wantread"
    end
    if not sdone then
      sok, serr = s:dohandshake()
      sdone = sok or serr ~= "wantread"
    end
    if cdone and sdone then break end
    socket.sleep(0.002)
  end
  c:settimeout(5); s:settimeout(5)
  return cok, cerr, sok, serr
end
local function server(extra)
  local p = { mode = "server", protocol = "any", key = dir .. "/server.key", certificate = dir .. "/server.pem" }
  for k, v in pairs(extra or {}) do p[k] = v end
  return p
end
local function client(extra)
  local p = { mode = "client", protocol = "any" }
  for k, v in pairs(extra or {}) do p[k] = v end
  return p
end
`

func TestSSLConnection(t *testing.T) {
	dir := t.TempDir()
	testPKI(t, dir)
	runIn(t, map[string]string{"dir": dir}, sslHelpers+`
assert(ssl._VERSION == "1.3.2" and ssl.config.protocols.tlsv1_3 and ssl.config.capabilities.alpn)

-- A client that asks for no checking connects, and is told what it would
-- have found: the server's CA is not one it knows.
local c, s = pair()
local cs = assert(ssl.wrap(c, client({ alpn = { "gemini", "h2" } })))
eq(c:getfd(), -1, "the plain socket gave its descriptor up")
assert(cs:getfd() >= 0)
cs:sni("example.test")
local ss = assert(ssl.wrap(s, server({ alpn = function(offered) return offered[1] end })))
assert(tostring(cs):find("^SSL connection: "))
local cok, cerr, sok, serr = handshake(cs, ss)
assert(cok, cerr); assert(sok, serr)
local ok, why = cs:getpeerverification()
eq(ok, false); eq(why, "unable to get local issuer certificate")
eq(cs:getalpn(), "gemini"); eq(ss:getalpn(), "gemini")
eq(ss:getsniname(), "example.test", "the name the client asked for")
eq(cs:getsniname(), "example.test")
local info = cs:info()
assert(info.protocol == "TLSv1.3" and info.cipher and info.bits > 0, info.protocol)
eq(cs:info("protocol"), "TLSv1.3")

-- Its certificate, as LuaSec describes certificates.
local cert = assert(cs:getpeercertificate())
local subject = cert:subject()
eq(subject[1].name, "C"); eq(subject[1].value, "DE"); eq(subject[2].name, "CN"); eq(subject[2].oid, "2.5.4.3")
eq(cert:issuer()[1].value, "tlua")
eq(cert:serial(), "BEEF")
eq(#cert:digest("sha256"), 64); eq(cert:digest(), cert:digest("sha1"))
eq(select(2, cert:digest("md5")), "digest algorithm not supported (md5)")
assert(cert:notbefore():match("^%a%a%a [ %d]%d %d%d:%d%d:%d%d %d%d%d%d GMT$"), cert:notbefore())
assert(cert:validat(os.time()) and not cert:validat(os.time() + 3 * 86400))
local san = cert:extensions()["2.5.29.17"]
eq(san.dNSName[1], "localhost"); eq(san.dNSName[2], "example.test"); eq(san.iPAddress[1], "127.0.0.1")
local pub, kind, bits = cert:pubkey()
assert(pub:find("BEGIN PUBLIC KEY")); eq(kind, "EC"); eq(bits, 256)
eq(cert:getsignaturename(), "ecdsa-with-SHA256")
local again = assert(ssl.loadcertificate(cert:pem()))
eq(again:digest("sha256"), cert:digest("sha256"))
local ca = assert(ssl.loadcertificate(io.open(dir .. "/ca.pem"):read("*a")))
assert(ca:issued(cert), "the CA issued it")
eq(#cs:getpeerchain(), 1)
eq(ss:getpeercertificate(), nil, "the client showed none")
eq(ss:getlocalcertificate():serial(), "BEEF")

-- Data both ways, with LuaSocket's receive patterns.
assert(cs:send("hello\r\nworld\n12345partial"))
eq(ss:receive(), "hello"); eq(ss:receive("*l"), "world"); eq(ss:receive(5), "12345")
assert(ss:dirty())
ss:settimeout(0.05)
local data, err, partial = ss:receive("*a")
eq(data, nil); eq(err, "wantread"); eq(partial, "partial")
ss:send("back\n")
local r = socket.select({ cs }, nil, 2)
eq(r[1], cs, "select sees TLS data")
eq(cs:receive(), "back")
eq(cs:receive(3, "pre"), "pre", "a prefix as long as asked for needs nothing read")
local big = ("x"):rep(100000)
assert(cs:send(big))
ss:settimeout(5)
eq(#assert(ss:receive(100000)), 100000)
local received, sent = cs:getstats()
assert(sent > 100000 and received == 5, received)

-- Closing ends the other side's read.
cs:close()
assert(tostring(cs):find("closed"))
eq(select(2, ss:receive()), "closed")
eq(select(2, cs:send("x")), "closed")
ss:close()

-- Asked to check, a client fails the handshake on a chain it cannot
-- follow, and connects with the CA given.
c, s = pair()
cs = assert(ssl.wrap(c, client({ verify = "peer" })))
ss = assert(ssl.wrap(s, server()))
cok, cerr = handshake(cs, ss)
eq(cok, false); eq(cerr, "certificate verify failed")
cs:close(); ss:close()
c, s = pair()
cs = assert(ssl.wrap(c, client({ verify = "peer", cafile = dir .. "/ca.pem" })))
ss = assert(ssl.wrap(s, server()))
assert(handshake(cs, ss))
eq(cs:getpeerverification(), true)
cs:close(); ss:close()

-- A self-signed server and a client certificate: Gemini's way. The client
-- trusts on first use, by the certificate's fingerprint.
c, s = pair()
cs = assert(ssl.wrap(c, client({ key = dir .. "/client.key", certificate = dir .. "/client.pem" })))
ss = assert(ssl.wrap(s, server({ key = dir .. "/self.key", certificate = dir .. "/self.pem",
  verify = { "peer" }, verifyext = { "lsec_continue" } })))
cok, cerr, sok, serr = handshake(cs, ss)
assert(cok, cerr); assert(sok, serr)
eq(select(2, cs:getpeerverification()), "self-signed certificate")
eq(#cs:getpeercertificate():digest("sha256"), 64)
eq(ss:getpeercertificate():subject()[1].value, "a reader", "the server sees the client's certificate")
cs:close(); ss:close()

-- Versions: a client that wants TLS 1.2 gets it.
c, s = pair()
cs = assert(ssl.wrap(c, client({ protocol = "tlsv1_2" })))
ss = assert(ssl.wrap(s, server()))
assert(handshake(cs, ss))
eq(cs:info("protocol"), "TLSv1.2")
assert(cs:info().cipher:find("^ECDHE%-ECDSA"), cs:info().cipher)
cs:close(); ss:close()

-- What newcontext refuses.
eq(select(2, ssl.newcontext({ mode = "client", protocol = "sslv2" })), "invalid protocol (sslv2)")
eq(select(2, ssl.newcontext({ mode = "client", protocol = "any", verify = "everything" })), "invalid verify option (everything)")
eq(select(2, ssl.newcontext({ mode = "client", protocol = "any", options = { "nonsense" } })), "invalid option (nonsense)")
assert(select(2, ssl.newcontext({ mode = "server", protocol = "any", key = dir .. "/missing.key" })):find("error loading private key"))
assert(not ssl.newcontext({ mode = "server", protocol = "any", key = dir .. "/server.key", certificate = dir .. "/self.pem" }),
  "a key that is not the certificate's")
assert(ssl.newcontext({ mode = "client", protocol = "any", curve = "prime256v1", ciphers = "ALL" }))
`)
}

// ssl.https fetches from a Go TLS server; socket.http does, through it.
func TestSSLHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Seen", r.Method+" "+string(body))
		w.Write([]byte("hello over TLS"))
	}))
	defer srv.Close()
	runIn(t, map[string]string{"url": srv.URL}, `
		local https = require "ssl.https"
		local body, code, headers = https.request(url .. "/x")
		assert(body == "hello over TLS" and code == 200, tostring(code))
		assert(headers["x-seen"] == "GET", headers["x-seen"])
		body, code, headers = https.request(url, "a=1")
		assert(code == 200 and headers["x-seen"] == "POST a=1")
		local http = require "socket.http"
		body, code = http.request(url .. "/y")
		assert(body == "hello over TLS" and code == 200, tostring(code))
		-- Asked to check, it refuses the test server, whose CA it does not know.
		local ok, err = https.request{ url = url .. "/z", verify = "peer" }
		assert(ok == nil and err == "certificate verify failed", tostring(err))
	`)
}

// A Go server that wants a client certificate gets the one a Lua client
// shows, and the Lua client reads what it sends back.
func TestSSLClientCertificate(t *testing.T) {
	dir := t.TempDir()
	testPKI(t, dir)
	cert, err := tls.LoadX509KeyPair(filepath.Join(dir, "self.pem"), filepath.Join(dir, "self.key"))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAnyClientCert})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- err.Error()
			return
		}
		defer conn.Close()
		tc := conn.(*tls.Conn)
		if err := tc.Handshake(); err != nil {
			got <- err.Error()
			return
		}
		line := make([]byte, 64)
		n, _ := tc.Read(line)
		peers := tc.ConnectionState().PeerCertificates
		name := ""
		if len(peers) > 0 {
			name = peers[0].Subject.CommonName
		}
		got <- name + "|" + string(line[:n])
		tc.Write([]byte("20 text/gemini\r\n# Hi\n"))
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	runIn(t, map[string]string{"dir": dir, "port": port}, `
		local socket, ssl = require "socket", require "ssl"
		local conn = assert(socket.connect("127.0.0.1", tonumber(port)))
		conn = assert(ssl.wrap(conn, { mode = "client", protocol = "tlsv1_2",
		  key = dir .. "/client.key", certificate = dir .. "/client.pem" }))
		conn:sni("capsule.test")
		conn:settimeout(5)
		assert(conn:dohandshake())
		assert(conn:send("gemini://capsule.test/\r\n"))
		assert(conn:receive() == "20 text/gemini")
		assert(conn:receive("*a") == "# Hi\n")
		conn:close()
	`)
	if s := <-got; s != "a reader|gemini://capsule.test/\r\n" {
		t.Errorf("the server got %q", s)
	}
}

//go:build linux || darwin

package lualib

// ssl.core, ssl.context, ssl.x509 and ssl.config: the C half of LuaSec
// 1.3.2, in Go, on crypto/tls. LuaSec's own ssl.lua and ssl/https.lua run
// unchanged on top (lua/luasec), as socket.lua does on socket.core:
//
//	local ssl = require "ssl"
//	local conn = ssl.wrap(tcp, { mode = "client", protocol = "any" })
//	conn:sni("example.org")
//	assert(conn:dohandshake())
//
// A connection takes over the descriptor of the LuaSocket object it wraps
// and reads and writes through a buffer of its own, with LuaSocket's
// receive patterns and LuaSec's results: a read or a write that runs out of
// time says "wantread" or "wantwrite", as OpenSSL does, not "timeout".
//
// Where OpenSSL and Go differ:
//
//   - The handshake runs on a goroutine, and dohandshake waits for it as
//     long as the timeout allows, returning false, "wantread" when it has
//     not finished, as a non-blocking OpenSSL handshake does. A Lua
//     callback it needs (a server's ALPN choice) runs while dohandshake
//     waits.
//   - Certificates are checked as OpenSSL checks them: the chain, and not
//     the host name, which LuaSec leaves to the program. verify = "peer"
//     fails the handshake on a bad chain; without it the handshake goes on,
//     and getpeerverification says what was wrong. With no cafile or
//     capath, the system's trusted roots are used; OpenSSL, as LuaSec sets
//     it up, would trust none.
//   - Cipher lists, DH parameters and the like are accepted and left to Go,
//     which picks its own ciphers. getfinished and getpeerfinished return
//     nothing, and there is no PSK or DANE.
//   - Keys are PEM: PKCS#1, PKCS#8 or EC, and the old encrypted PEM with a
//     password; not encrypted PKCS#8. A key or certificate parameter can be
//     the PEM itself as well as a file's path, and a path is read out of a
//     fused program or a bundle first, as png.load reads.

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const (
	classSSLConn = "SSL:Connection"
	classSSLCtx  = "SSL:Context"
	classSSLCert = "SSL:Certificate"
)

// What an SSL operation came to, beyond socket.core's: it ran out of time
// waiting to read or to write, or TLS itself failed, as lastErr says.
const (
	ioWantRead  = -10
	ioWantWrite = -11
	ioSSL       = -12
)

func preloadSSL(L *lua.LState) {
	L.PreloadModule("ssl.core", openSSLCore)
	L.PreloadModule("ssl.context", openSSLContext)
	L.PreloadModule("ssl.x509", openSSLX509)
	L.PreloadModule("ssl.config", openSSLConfig)
}

// ---------------------------------------------------------------- ssl.config

// sslOptions are the names OpenSSL's options go by, which setoptions
// accepts; those that limit the protocol versions are followed, the rest
// are Go's to decide.
var sslOptions = strings.Fields(`all allow_client_renegotiation allow_no_dhe_kex
	allow_unsafe_legacy_renegotiation cipher_server_preference cisco_anyconnect
	cleanse_plaintext cookie_exchange cryptopro_tlsext_bug disable_tlsext_ca_names
	dont_insert_empty_fragments enable_ktls enable_middlebox_compat ephemeral_rsa
	ignore_unexpected_eof legacy_server_connect microsoft_big_sslv3_buffer
	microsoft_sess_id_bug msie_sslv2_rsa_padding netscape_ca_dn_bug
	netscape_challenge_bug netscape_demo_cipher_change_bug
	netscape_reuse_cipher_change_bug no_anti_replay no_compression no_dtls_mask
	no_dtlsv1 no_dtlsv1_2 no_encrypt_then_mac no_extended_master_secret
	no_query_mtu no_renegotiation no_session_resumption_on_renegotiation
	no_ssl_mask no_sslv2 no_sslv3 no_ticket no_tlsv1 no_tlsv1_1 no_tlsv1_2
	no_tlsv1_3 pkcs1_check_1 pkcs1_check_2 prioritize_chacha
	safari_ecdhe_ecdsa_bug single_dh_use single_ecdh_use ssleay_080_client_dh_bug
	sslref2_reuse_cert_type_bug tlsext_padding tls_block_padding_bug tls_d5_bug
	tls_rollback_bug`)

// sslCurves are the elliptic curves Go has, by OpenSSL's names and its
// NIDs, as config.curves lists them.
var sslCurves = map[string]struct {
	id  tls.CurveID
	nid int
}{
	"prime256v1": {tls.CurveP256, 415},
	"secp384r1":  {tls.CurveP384, 715},
	"secp521r1":  {tls.CurveP521, 716},
	"X25519":     {tls.X25519, 1034},
}

// curveNamed reads a curve's name, OpenSSL's or the NIST one.
func curveNamed(name string) (tls.CurveID, bool) {
	switch name {
	case "P-256":
		name = "prime256v1"
	case "P-384":
		name = "secp384r1"
	case "P-521":
		name = "secp521r1"
	case "x25519":
		name = "X25519"
	}
	c, ok := sslCurves[name]
	return c.id, ok
}

func openSSLConfig(L *lua.LState) int {
	set := func(names ...string) *lua.LTable {
		t := L.NewTable()
		for _, n := range names {
			t.RawSetString(n, lua.LTrue)
		}
		return t
	}
	cfg := L.NewTable()
	cfg.RawSetString("options", set(sslOptions...))
	cfg.RawSetString("protocols", set("tlsv1", "tlsv1_1", "tlsv1_2", "tlsv1_3"))
	cfg.RawSetString("algorithms", set("ec"))
	curves := L.NewTable()
	for name, c := range sslCurves {
		curves.RawSetString(name, lua.LNumber(c.nid))
	}
	cfg.RawSetString("curves", curves)
	cfg.RawSetString("capabilities", set("alpn", "curves_list", "ecdh_auto"))
	L.Push(cfg)
	return 1
}

// ---------------------------------------------------------------- ssl.context

// sslContext is what newcontext sets up, for the connections made with it.
type sslContext struct {
	mode       string // "client", "server", or "" until setmode
	minV, maxV uint16 // from the protocol; 0 for Go's choice
	noVersion  map[uint16]bool
	certs      []tls.Certificate
	key        crypto.PrivateKey // loaded, waiting for its certificate
	chain      [][]byte          // loaded, waiting for its key
	roots      *x509.CertPool    // nil: the system's
	verifyPeer bool
	failNoPeer bool
	depth      int
	alpn       []string       // a client's
	alpnCB     *lua.LFunction // a server's
	curves     []tls.CurveID
	keepGoing  bool // verifyext lsec_continue
	anyPurpose bool // verifyext lsec_ignore_purpose
}

func checkCtx(L *lua.LState, n int) *sslContext {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if c, ok := ud.Value.(*sslContext); ok {
			return c
		}
	}
	L.ArgError(n, "SSL:Context expected")
	return nil
}

func ctxOK(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

func ctxFail(L *lua.LState, format string, args ...any) int {
	L.Push(lua.LFalse)
	L.Push(lua.LString(fmt.Sprintf(format, args...)))
	return 2
}

func openSSLContext(L *lua.LState) int {
	mt := L.NewTypeMetatable(classSSLCtx)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"setverifyext": ctxSetVerifyExt,
	}))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(fmt.Sprintf("SSL context: %p", checkCtx(L, 1))))
		return 1
	}))
	L.Push(L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"create":          ctxCreate,
		"locations":       ctxLocations,
		"loadcert":        ctxLoadCert,
		"loadkey":         ctxLoadKey,
		"checkkey":        ctxCheckKey,
		"setalpn":         ctxSetALPN,
		"setalpncb":       ctxSetALPNCB,
		"setcipher":       ctxOK,
		"setciphersuites": ctxOK,
		"setdepth":        ctxSetDepth,
		"setdhparam":      func(L *lua.LState) int { return 0 },
		"setverify":       ctxSetVerify,
		"setoptions":      ctxSetOptions,
		"setmode":         ctxSetMode,
		"setcurve":        ctxSetCurve,
		"setcurveslist":   ctxSetCurvesList,
	}))
	return 1
}

func ctxCreate(L *lua.LState) int {
	proto := L.CheckString(1)
	c := &sslContext{depth: -1, noVersion: map[uint16]bool{}}
	switch proto {
	case "any", "sslv23":
	case "tlsv1":
		c.minV, c.maxV = tls.VersionTLS10, tls.VersionTLS10
	case "tlsv1_1":
		c.minV, c.maxV = tls.VersionTLS11, tls.VersionTLS11
	case "tlsv1_2":
		c.minV, c.maxV = tls.VersionTLS12, tls.VersionTLS12
	case "tlsv1_3":
		c.minV, c.maxV = tls.VersionTLS13, tls.VersionTLS13
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid protocol (" + proto + ")"))
		return 2
	}
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(classSSLCtx))
	L.Push(ud)
	return 1
}

func ctxSetMode(L *lua.LState) int {
	c := checkCtx(L, 1)
	mode := L.CheckString(2)
	if mode != "client" && mode != "server" {
		return ctxFail(L, "invalid mode (%s)", mode)
	}
	c.mode = mode
	return ctxOK(L)
}

// pemData is a key's or a certificate's PEM: the text itself, or a file's.
func pemData(L *lua.LState, arg string) ([]byte, error) {
	if strings.Contains(arg, "-----BEGIN ") {
		return []byte(arg), nil
	}
	return readFile(L, arg)
}

// pair makes a certificate of a key and the chain loaded for it, once both
// are there.
func (c *sslContext) pair() {
	if c.key == nil || c.chain == nil {
		return
	}
	cert := tls.Certificate{Certificate: c.chain, PrivateKey: c.key}
	if leaf, err := x509.ParseCertificate(c.chain[0]); err == nil {
		cert.Leaf = leaf
	}
	c.certs = append(c.certs, cert)
	c.key, c.chain = nil, nil
}

func ctxLoadCert(L *lua.LState) int {
	c := checkCtx(L, 1)
	data, err := pemData(L, L.CheckString(2))
	if err != nil {
		return ctxFail(L, "error loading certificate (%s)", sysReason(err))
	}
	var chain [][]byte
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				return ctxFail(L, "error loading certificate (%s)", err.Error())
			}
			chain = append(chain, block.Bytes)
		}
	}
	if len(chain) == 0 {
		return ctxFail(L, "error loading certificate (no start line)")
	}
	c.chain = chain
	c.pair()
	return ctxOK(L)
}

func ctxLoadKey(L *lua.LState) int {
	c := checkCtx(L, 1)
	data, err := pemData(L, L.CheckString(2))
	var password func() string
	switch pw := L.Get(3).(type) {
	case *lua.LNilType:
	case lua.LString:
		password = func() string { return string(pw) }
	case *lua.LFunction:
		password = func() string {
			L.Push(pw)
			L.Call(0, 1)
			s := L.ToString(-1)
			L.Pop(1)
			return s
		}
	default:
		L.RaiseError("invalid callback value")
	}
	if err != nil {
		return ctxFail(L, "error loading private key (%s)", sysReason(err))
	}
	key, err := parseKey(data, password)
	if err != nil {
		return ctxFail(L, "error loading private key (%s)", err.Error())
	}
	c.key = key
	c.pair()
	return ctxOK(L)
}

// parseKey reads the first private key in PEM data.
func parseKey(data []byte, password func() string) (crypto.PrivateKey, error) {
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			return nil, errors.New("no start line")
		}
		data = rest
		if !strings.HasSuffix(block.Type, "PRIVATE KEY") {
			continue
		}
		if block.Type == "ENCRYPTED PRIVATE KEY" {
			return nil, errors.New("encrypted PKCS#8 keys are not supported; decrypt it, or use the old encrypted PEM")
		}
		der := block.Bytes
		//lint:ignore SA1019 the old encrypted PEM is what OpenSSL's password callback reads
		if x509.IsEncryptedPEMBlock(block) {
			if password == nil {
				return nil, errors.New("the key is encrypted, and no password was given")
			}
			var err error
			//lint:ignore SA1019 as above
			if der, err = x509.DecryptPEMBlock(block, []byte(password())); err != nil {
				return nil, errors.New("bad decrypt")
			}
		}
		if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
			return k, nil
		}
		if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
			return k, nil
		}
		if k, err := x509.ParseECPrivateKey(der); err == nil {
			return k, nil
		}
		return nil, errors.New("unsupported private key")
	}
}

// ctxCheckKey says whether the last key and certificate loaded belong
// together.
func ctxCheckKey(L *lua.LState) int {
	c := checkCtx(L, 1)
	ok := false
	if n := len(c.certs); n > 0 && c.certs[n-1].Leaf != nil {
		cert := c.certs[n-1]
		if signer, isSigner := cert.PrivateKey.(crypto.Signer); isSigner {
			if pub, isEq := signer.Public().(interface{ Equal(crypto.PublicKey) bool }); isEq {
				ok = pub.Equal(cert.Leaf.PublicKey)
			}
		}
	}
	L.Push(lua.LBool(ok))
	return 1
}

func ctxLocations(L *lua.LState) int {
	c := checkCtx(L, 1)
	cafile, capath := L.OptString(2, ""), L.OptString(3, "")
	pool := c.roots
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if cafile != "" {
		data, err := pemData(L, cafile)
		if err != nil {
			return ctxFail(L, "error loading CA locations (%s)", sysReason(err))
		}
		if !pool.AppendCertsFromPEM(data) {
			return ctxFail(L, "error loading CA locations (no certificate or crl found)")
		}
	}
	if capath != "" {
		entries, err := os.ReadDir(capath)
		if err != nil {
			return ctxFail(L, "error loading CA locations (%s)", sysReason(err))
		}
		for _, e := range entries {
			if data, err := os.ReadFile(filepath.Join(capath, e.Name())); err == nil {
				pool.AppendCertsFromPEM(data)
			}
		}
	}
	c.roots = pool
	return ctxOK(L)
}

func ctxSetDepth(L *lua.LState) int {
	checkCtx(L, 1).depth = L.CheckInt(2)
	return ctxOK(L)
}

func ctxSetVerify(L *lua.LState) int {
	c := checkCtx(L, 1)
	for i := 2; i <= L.GetTop(); i++ {
		switch flag := L.CheckString(i); flag {
		case "none", "client_once":
		case "peer":
			c.verifyPeer = true
		case "fail_if_no_peer_cert":
			c.failNoPeer = true
		default:
			return ctxFail(L, "invalid verify option (%s)", flag)
		}
	}
	return ctxOK(L)
}

func ctxSetVerifyExt(L *lua.LState) int {
	c := checkCtx(L, 1)
	keepGoing, anyPurpose := false, false
	for i := 2; i <= L.GetTop(); i++ {
		switch flag := L.CheckString(i); flag {
		case "lsec_continue":
			keepGoing = true
		case "lsec_ignore_purpose":
			anyPurpose = true
		case "crl_check", "crl_check_chain":
		default:
			return ctxFail(L, "invalid verify option (%s)", flag)
		}
	}
	c.keepGoing, c.anyPurpose = keepGoing, anyPurpose
	return ctxOK(L)
}

func ctxSetOptions(L *lua.LState) int {
	c := checkCtx(L, 1)
	known := map[string]bool{}
	for _, o := range sslOptions {
		known[o] = true
	}
	for i := 2; i <= L.GetTop(); i++ {
		opt := L.CheckString(i)
		if !known[opt] {
			return ctxFail(L, "invalid option (%s)", opt)
		}
		switch opt {
		case "no_tlsv1":
			c.noVersion[tls.VersionTLS10] = true
		case "no_tlsv1_1":
			c.noVersion[tls.VersionTLS11] = true
		case "no_tlsv1_2":
			c.noVersion[tls.VersionTLS12] = true
		case "no_tlsv1_3":
			c.noVersion[tls.VersionTLS13] = true
		}
	}
	return ctxOK(L)
}

func ctxSetCurve(L *lua.LState) int {
	c := checkCtx(L, 1)
	name := L.CheckString(2)
	id, ok := curveNamed(name)
	if !ok {
		return ctxFail(L, "elliptic curve '%s' not supported", name)
	}
	c.curves = []tls.CurveID{id}
	return ctxOK(L)
}

func ctxSetCurvesList(L *lua.LState) int {
	c := checkCtx(L, 1)
	list := L.CheckString(2)
	var ids []tls.CurveID
	for _, name := range strings.Split(list, ":") {
		id, ok := curveNamed(strings.TrimSpace(name))
		if !ok {
			return ctxFail(L, "unknown elliptic curve in \"%s\"", list)
		}
		ids = append(ids, id)
	}
	c.curves = ids
	return ctxOK(L)
}

// wireProtocols reads ALPN's wire format: each name after its length.
func wireProtocols(wire string) []string {
	var out []string
	for i := 0; i < len(wire); {
		n := int(wire[i])
		if i+1+n > len(wire) {
			break
		}
		out = append(out, wire[i+1:i+1+n])
		i += 1 + n
	}
	return out
}

func toWire(protos []string) string {
	var b strings.Builder
	for _, p := range protos {
		b.WriteByte(byte(len(p)))
		b.WriteString(p)
	}
	return b.String()
}

func ctxSetALPN(L *lua.LState) int {
	c := checkCtx(L, 1)
	c.alpn = wireProtocols(L.CheckString(2))
	return ctxOK(L)
}

func ctxSetALPNCB(L *lua.LState) int {
	c := checkCtx(L, 1)
	c.alpnCB = L.CheckFunction(2)
	return ctxOK(L)
}

// versions is the range of TLS versions a context allows: its protocol's,
// less those its options rule out.
func (c *sslContext) versions() (uint16, uint16) {
	lo, hi := c.minV, c.maxV
	if lo == 0 {
		lo = tls.VersionTLS12 // Go's own floor, for "any"
	}
	if hi == 0 {
		hi = tls.VersionTLS13
	}
	for lo <= hi && c.noVersion[lo] {
		lo++
	}
	for hi >= lo && c.noVersion[hi] {
		hi--
	}
	return lo, hi
}

// ---------------------------------------------------------------- ssl.x509

type sslCert struct {
	cert *x509.Certificate
}

func pushCert(L *lua.LState, cert *x509.Certificate) {
	ud := L.NewUserData()
	ud.Value = &sslCert{cert: cert}
	L.SetMetatable(ud, L.GetTypeMetatable(classSSLCert))
	L.Push(ud)
}

func checkCert(L *lua.LState, n int) *x509.Certificate {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if c, ok := ud.Value.(*sslCert); ok {
			return c.cert
		}
	}
	L.ArgError(n, "SSL:Certificate expected")
	return nil
}

// registerCertClass makes the certificates' metatable; ssl.core needs it
// too, for the certificates a connection hands out.
func registerCertClass(L *lua.LState) {
	if L.GetTypeMetatable(classSSLCert) != lua.LNil {
		return
	}
	mt := L.NewTypeMetatable(classSSLCert)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"digest":           certDigest,
		"setencode":        certSetEncode,
		"extensions":       certExtensions,
		"getsignaturename": certSignatureName,
		"issuer":           func(L *lua.LState) int { return pushX509Name(L, checkCert(L, 1).Issuer) },
		"subject":          func(L *lua.LState) int { return pushX509Name(L, checkCert(L, 1).Subject) },
		"notbefore":        func(L *lua.LState) int { return pushTime(L, checkCert(L, 1).NotBefore) },
		"notafter":         func(L *lua.LState) int { return pushTime(L, checkCert(L, 1).NotAfter) },
		"issued":           certIssued,
		"pem":              certPEM,
		"pubkey":           certPubkey,
		"serial":           certSerial,
		"validat":          certValidAt,
	}))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(fmt.Sprintf("X509 certificate: %p", checkCert(L, 1))))
		return 1
	}))
}

func openSSLX509(L *lua.LState) int {
	registerCertClass(L)
	L.Push(L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"load": func(L *lua.LState) int {
			block, _ := pem.Decode([]byte(L.CheckString(1)))
			for block != nil && block.Type != "CERTIFICATE" {
				block = nil
			}
			if block == nil {
				L.Push(lua.LNil)
				return 1
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				L.Push(lua.LNil)
				return 1
			}
			pushCert(L, cert)
			return 1
		},
	}))
	return 1
}

// nameOIDs are OpenSSL's short names for the attributes of a name.
var nameOIDs = map[string]string{
	"2.5.4.3": "CN", "2.5.4.4": "SN", "2.5.4.5": "serialNumber", "2.5.4.6": "C",
	"2.5.4.7": "L", "2.5.4.8": "ST", "2.5.4.9": "street", "2.5.4.10": "O",
	"2.5.4.11": "OU", "2.5.4.12": "title", "2.5.4.17": "postalCode",
	"2.5.4.42": "GN", "2.5.4.43": "initials", "2.5.4.46": "dnQualifier",
	"1.2.840.113549.1.9.1":       "emailAddress",
	"0.9.2342.19200300.100.1.1":  "UID",
	"0.9.2342.19200300.100.1.25": "DC",
}

// pushX509Name is a name as LuaSec lists it: each attribute's oid, short name
// and value, in order.
func pushX509Name(L *lua.LState, name pkix.Name) int {
	t := L.NewTable()
	for _, atv := range name.Names {
		oid := atv.Type.String()
		e := L.NewTable()
		e.RawSetString("oid", lua.LString(oid))
		short, ok := nameOIDs[oid]
		if !ok {
			short = oid
		}
		e.RawSetString("name", lua.LString(short))
		e.RawSetString("value", lua.LString(fmt.Sprint(atv.Value)))
		t.Append(e)
	}
	L.Push(t)
	return 1
}

// pushTime is a time as OpenSSL prints it: "Jan  2 15:04:05 2026 GMT".
func pushTime(L *lua.LState, t time.Time) int {
	L.Push(lua.LString(t.UTC().Format("Jan _2 15:04:05 2006 GMT")))
	return 1
}

func certDigest(L *lua.LState) int {
	cert := checkCert(L, 1)
	alg := L.OptString(2, "sha1")
	var h hash.Hash
	switch alg {
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		L.Push(lua.LNil)
		L.Push(lua.LString("digest algorithm not supported (" + alg + ")"))
		return 2
	}
	h.Write(cert.Raw)
	L.Push(lua.LString(hex.EncodeToString(h.Sum(nil))))
	return 1
}

// certSetEncode chooses how names' values come out; Go has them as UTF-8
// either way.
func certSetEncode(L *lua.LState) int {
	checkCert(L, 1)
	enc := L.CheckString(2)
	L.Push(lua.LBool(strings.HasPrefix(enc, "ai5") || strings.HasPrefix(enc, "utf8")))
	return 1
}

// certExtensions is the subject's alternative names, as LuaSec gives
// them: by the extension's oid, each kind of name a list.
func certExtensions(L *lua.LState) int {
	cert := checkCert(L, 1)
	out := L.NewTable()
	has := false
	for _, ext := range cert.Extensions {
		if ext.Id.String() == "2.5.29.17" {
			has = true
		}
	}
	if has {
		san := L.NewTable()
		san.RawSetString("name", lua.LString("X509v3 Subject Alternative Name"))
		list := func(key string, values []string) {
			if len(values) == 0 {
				return
			}
			t := L.NewTable()
			for _, v := range values {
				t.Append(lua.LString(v))
			}
			san.RawSetString(key, t)
		}
		list("dNSName", cert.DNSNames)
		list("rfc822Name", cert.EmailAddresses)
		var uris, ips []string
		for _, u := range cert.URIs {
			uris = append(uris, u.String())
		}
		for _, ip := range cert.IPAddresses {
			ips = append(ips, ip.String())
		}
		list("uniformResourceIdentifier", uris)
		list("iPAddress", ips)
		out.RawSetString("2.5.29.17", san)
	}
	L.Push(out)
	return 1
}

func certSignatureName(L *lua.LState) int {
	names := map[x509.SignatureAlgorithm]string{
		x509.SHA1WithRSA: "RSA-SHA1", x509.SHA256WithRSA: "RSA-SHA256",
		x509.SHA384WithRSA: "RSA-SHA384", x509.SHA512WithRSA: "RSA-SHA512",
		x509.SHA256WithRSAPSS: "RSASSA-PSS", x509.SHA384WithRSAPSS: "RSASSA-PSS",
		x509.SHA512WithRSAPSS: "RSASSA-PSS", x509.ECDSAWithSHA1: "ecdsa-with-SHA1",
		x509.ECDSAWithSHA256: "ecdsa-with-SHA256", x509.ECDSAWithSHA384: "ecdsa-with-SHA384",
		x509.ECDSAWithSHA512: "ecdsa-with-SHA512", x509.PureEd25519: "ED25519",
	}
	if name, ok := names[checkCert(L, 1).SignatureAlgorithm]; ok {
		L.Push(lua.LString(name))
	} else {
		L.Push(lua.LNil)
	}
	return 1
}

func certPEM(L *lua.LState) int {
	cert := checkCert(L, 1)
	L.Push(lua.LString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})))
	return 1
}

// certPubkey is the public key as PEM, its kind and its size in bits.
func certPubkey(L *lua.LState) int {
	cert := checkCert(L, 1)
	der, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))
	switch k := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		L.Push(lua.LString("RSA"))
		L.Push(lua.LNumber(k.N.BitLen()))
	case *ecdsa.PublicKey:
		L.Push(lua.LString("EC"))
		L.Push(lua.LNumber(k.Curve.Params().BitSize))
	case ed25519.PublicKey:
		L.Push(lua.LString("Unknown"))
		L.Push(lua.LNumber(253))
	default:
		L.Push(lua.LString("Unknown"))
		L.Push(lua.LNumber(0))
	}
	return 3
}

// certSerial is the serial number in hexadecimal, as OpenSSL writes it.
func certSerial(L *lua.LState) int {
	L.Push(lua.LString(strings.ToUpper(checkCert(L, 1).SerialNumber.Text(16))))
	return 1
}

// certValidAt says whether the certificate is valid at a time, seconds
// since the epoch.
func certValidAt(L *lua.LState) int {
	cert := checkCert(L, 1)
	t := int64(L.CheckNumber(2))
	L.Push(lua.LBool(cert.NotBefore.Unix() <= t && t <= cert.NotAfter.Unix()))
	return 1
}

// certIssued is issuer:issued(subject [, intermediate...]): whether the
// subject's chain leads to the issuer.
func certIssued(L *lua.LState) int {
	issuer, subject := checkCert(L, 1), checkCert(L, 2)
	roots, inter := x509.NewCertPool(), x509.NewCertPool()
	roots.AddCert(issuer)
	for i := 3; i <= L.GetTop(); i++ {
		inter.AddCert(checkCert(L, i))
	}
	_, err := subject.Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}, CurrentTime: subject.NotBefore.Add(time.Second)})
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(verifyReason(err, []*x509.Certificate{subject})))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}

// ---------------------------------------------------------------- ssl.core

const (
	sslNew = iota
	sslConnected
	sslClosed
)

// sslConn is one connection: LuaSec's t_ssl.
type sslConn struct {
	ctx    *sslContext
	state  int
	fd     int // what getfd reports: nc's descriptor
	nc     net.Conn
	tc     *tls.Conn
	ncErr  error // why the descriptor could not be taken over
	buf    sockBuffer
	tm     timeout
	sni    string                 // a client's server name
	sniMap map[string]*sslContext // a server's contexts by name
	strict bool                   // a server with sniMap refuses other names
	served string                 // the name a client asked a server for
	hs     chan error             // a handshake under way
	hsErr  error                  // how it ended
	calls  chan func(*lua.LState) // work the handshake has for the Lua thread
	quit   chan struct{}          // closed when the connection is
	write  chan error             // a write under way
	sticky error                  // what a read found while dirty only looked
	reason string                 // ioSSL's message
	verify string                 // what was wrong with the peer's chain; "" for nothing
	peers  []*x509.Certificate
	want   string
}

func checkConn(L *lua.LState) *sslConn {
	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if c, ok := ud.Value.(*sslConn); ok {
			return c
		}
	}
	L.ArgError(1, classSSLConn+" expected")
	return nil
}

func openSSLCore(L *lua.LState) int {
	registerCertClass(L)
	mt := L.NewTypeMetatable(classSSLConn)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"close":                connClose,
		"getalpn":              connGetALPN,
		"getfd":                func(L *lua.LState) int { L.Push(lua.LNumber(checkConn(L).fd)); return 1 },
		"getfinished":          func(L *lua.LState) int { checkConn(L); return 0 },
		"getpeerfinished":      func(L *lua.LState) int { checkConn(L); return 0 },
		"getpeercertificate":   connPeerCert,
		"getlocalcertificate":  connLocalCert,
		"getpeerchain":         connPeerChain,
		"getlocalchain":        connLocalChain,
		"getpeerverification":  connPeerVerification,
		"exportkeyingmaterial": connExportKeyingMaterial,
		"getsniname":           connGetSNIName,
		"getstats":             func(L *lua.LState) int { return bufGetstats(L, &checkConn(L).buf) },
		"setstats":             func(L *lua.LState) int { return bufSetstats(L, &checkConn(L).buf) },
		"dirty":                connDirty,
		"dohandshake":          connHandshake,
		"receive":              connReceive,
		"send":                 connSend,
		"settimeout":           connSettimeout,
		"sni":                  connSNI,
		"want":                 connWant,
	}))
	L.SetField(mt, "__gc", L.NewFunction(connClose))
	L.SetField(mt, "__close", L.NewFunction(connClose))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		c := checkConn(L)
		closed := ""
		if c.state == sslClosed {
			closed = " (closed)"
		}
		L.Push(lua.LString(fmt.Sprintf("SSL connection: %p%s", c, closed)))
		return 1
	}))
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"compression": connCompression,
		"create":      connCreate,
		"info":        connInfo,
		"setfd":       connSetfd,
		"setmethod":   connSetMethod,
		"copyright": func(L *lua.LState) int {
			L.Push(lua.LString("LuaSec 1.3.2 - Copyright (C) 2006-2023 Bruno Silvestre, UFG"))
			return 1
		},
	})
	L.SetField(mod, "SOCKET_INVALID", lua.LNumber(sockInvalid))
	L.Push(mod)
	return 1
}

func connCreate(L *lua.LState) int {
	ctx := checkCtx(L, 1)
	if ctx.mode == "" {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid mode"))
		return 2
	}
	c := &sslConn{ctx: ctx, fd: sockInvalid, tm: newTimeout(), quit: make(chan struct{})}
	c.buf.birthday = gettime()
	runtime.SetFinalizer(c, func(c *sslConn) {
		if c.nc != nil {
			c.nc.Close()
		}
	})
	ud := L.NewUserData()
	ud.Value = c
	L.SetMetatable(ud, L.GetTypeMetatable(classSSLConn))
	L.Push(ud)
	return 1
}

// connSetfd is core.setfd(conn, fd): the connection takes the descriptor
// over, as a Go connection of its own, and the number it had is closed.
func connSetfd(L *lua.LState) int {
	c := checkConn(L)
	if c.state != sslNew {
		L.ArgError(1, "invalid SSL object state")
	}
	fd := L.CheckInt(2)
	if c.nc != nil {
		c.nc.Close()
		c.nc, c.fd = nil, sockInvalid
	}
	if fd < 0 {
		return 0
	}
	f := os.NewFile(uintptr(fd), "socket")
	nc, err := net.FileConn(f)
	f.Close()
	if err != nil {
		c.ncErr = err
		return 0
	}
	c.nc = nc
	if sc, ok := nc.(syscall.Conn); ok {
		if raw, err := sc.SyscallConn(); err == nil {
			raw.Control(func(d uintptr) { c.fd = int(d) })
		}
	}
	return 0
}

func connSetMethod(L *lua.LState) int {
	index := L.GetField(L.GetTypeMetatable(classSSLConn), "__index").(*lua.LTable)
	index.RawSet(L.Get(1), L.Get(2))
	return 0
}

func connSettimeout(L *lua.LState) int {
	c := checkConn(L)
	t := float64(L.OptNumber(2, -1))
	switch mode := optStringDef(L, 3, "b"); {
	case mode != "" && mode[0] == 'b':
		c.tm.block = t
	case mode != "" && (mode[0] == 'r' || mode[0] == 't'):
		c.tm.total = t
	default:
		L.ArgError(3, "invalid timeout mode")
	}
	return pushOne(L)
}

func connSNI(L *lua.LState) int {
	c := checkConn(L)
	if c.ctx.mode == "client" {
		c.sni = L.CheckString(2)
		return 0
	}
	tab := L.CheckTable(2)
	c.sniMap = map[string]*sslContext{}
	tab.ForEach(func(k, v lua.LValue) {
		name, ok := k.(lua.LString)
		if !ok {
			L.ArgError(2, "the names must be strings")
		}
		ud, isUD := v.(*lua.LUserData)
		ctx, isCtx := (*sslContext)(nil), false
		if isUD {
			ctx, isCtx = ud.Value.(*sslContext)
		}
		if !isCtx {
			L.ArgError(2, "SSL:Context expected for "+string(name))
		}
		c.sniMap[string(name)] = ctx
	})
	c.strict = L.ToBool(3)
	return 0
}

// config is the crypto/tls configuration for a context; the connection's
// own part (server name, verification) is added by start.
func (c *sslConn) config(ctx *sslContext) *tls.Config {
	lo, hi := ctx.versions()
	cfg := &tls.Config{
		MinVersion:       lo,
		MaxVersion:       hi,
		Certificates:     ctx.certs,
		CurvePreferences: ctx.curves,
		// Every chain is checked here, as OpenSSL checks it, and only
		// fails the handshake when the context says to.
		InsecureSkipVerify: true,
		VerifyConnection:   func(cs tls.ConnectionState) error { return c.verifyPeer(ctx, cs) },
	}
	if ctx.mode == "client" {
		cfg.ServerName = c.sni
		cfg.NextProtos = ctx.alpn
		if len(ctx.certs) > 0 {
			// OpenSSL sends the certificate whatever the server asks for.
			cert := &ctx.certs[0]
			cfg.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) { return cert, nil }
		}
		return cfg
	}
	switch {
	case ctx.verifyPeer && ctx.failNoPeer:
		cfg.ClientAuth = tls.RequireAnyClientCert
	case ctx.verifyPeer:
		cfg.ClientAuth = tls.RequestClientCert
	}
	return cfg
}

var errVerify = errors.New("certificate verify failed")

// verifyPeer checks the peer's chain against the context's roots, or the
// system's, and keeps what it found for getpeerverification.
func (c *sslConn) verifyPeer(ctx *sslContext, cs tls.ConnectionState) error {
	c.peers = cs.PeerCertificates
	c.verify = ""
	if len(cs.PeerCertificates) == 0 {
		if ctx.mode == "server" && ctx.verifyPeer && ctx.failNoPeer {
			return errors.New("peer did not return a certificate")
		}
		return nil
	}
	leaf := cs.PeerCertificates[0]
	inter := x509.NewCertPool()
	for _, cert := range cs.PeerCertificates[1:] {
		inter.AddCert(cert)
	}
	usage := x509.ExtKeyUsageServerAuth
	if ctx.mode == "server" {
		usage = x509.ExtKeyUsageClientAuth
	}
	if ctx.anyPurpose {
		usage = x509.ExtKeyUsageAny
	}
	chains, err := leaf.Verify(x509.VerifyOptions{Roots: ctx.roots, Intermediates: inter, KeyUsages: []x509.ExtKeyUsage{usage}})
	switch {
	case err != nil:
		c.verify = verifyReason(err, cs.PeerCertificates)
	case ctx.depth >= 0 && len(chains) > 0 && len(chains[0])-2 > ctx.depth:
		c.verify = "certificate chain too long"
	}
	if c.verify != "" && ctx.verifyPeer && !ctx.keepGoing {
		return errVerify
	}
	return nil
}

// verifyReason says what was wrong with a chain in OpenSSL's words, which
// programs compare against.
func verifyReason(err error, chain []*x509.Certificate) string {
	selfSigned := func(c *x509.Certificate) bool {
		// Signed by its own key, CA or not, as a capsule's often is.
		return bytes.Equal(c.RawIssuer, c.RawSubject) &&
			c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature) == nil
	}
	var unknown x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var sysRoots x509.SystemRootsError
	// macOS's own verifier, which Go asks when the roots are the system's,
	// says it in words of its own.
	msg := err.Error()
	untrusted := strings.Contains(msg, "is not trusted") || strings.Contains(msg, "unknown authority")
	switch {
	case !errors.As(err, &invalid) && strings.Contains(msg, "expired"):
		return "certificate has expired"
	case errors.As(err, &unknown) || errors.As(err, &sysRoots) || untrusted:
		switch {
		case len(chain) == 1 && selfSigned(chain[0]):
			return "self-signed certificate"
		case len(chain) > 1 && selfSigned(chain[len(chain)-1]):
			return "self-signed certificate in certificate chain"
		}
		return "unable to get local issuer certificate"
	case errors.As(err, &invalid):
		switch invalid.Reason {
		case x509.Expired:
			if time.Now().Before(invalid.Cert.NotBefore) {
				return "certificate is not yet valid"
			}
			return "certificate has expired"
		case x509.IncompatibleUsage:
			return "unsupported certificate purpose"
		case x509.CANotAuthorizedForThisName, x509.NameConstraintsWithoutSANs:
			return "permitted subtree violation"
		case x509.TooManyIntermediates:
			return "certificate chain too long"
		}
	}
	return err.Error()
}

// start begins the handshake on a goroutine.
func (c *sslConn) start() {
	if c.ctx.mode == "server" {
		base := c.config(c.ctx)
		base.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			c.served = hello.ServerName
			ctx := c.ctx
			if c.sniMap != nil && hello.ServerName != "" {
				if other, ok := c.sniMap[hello.ServerName]; ok {
					ctx = other
				} else if c.strict {
					return nil, errors.New("unrecognized name")
				}
			}
			cfg := c.config(ctx)
			if ctx.alpnCB != nil && len(hello.SupportedProtos) > 0 {
				chosen, err := c.alpnChoice(ctx.alpnCB, hello.SupportedProtos)
				if err != nil {
					return nil, err
				}
				cfg.NextProtos = chosen
			}
			return cfg, nil
		}
		c.tc = tls.Server(c.nc, base)
	} else {
		c.tc = tls.Client(c.nc, c.config(c.ctx))
	}
	c.hs = make(chan error, 1)
	c.calls = make(chan func(*lua.LState))
	tc, hs := c.tc, c.hs
	go func() { hs <- tc.Handshake() }()
}

// alpnChoice asks the server's Lua callback which protocol to speak, on
// the Lua thread, while dohandshake waits.
func (c *sslConn) alpnChoice(cb *lua.LFunction, offered []string) ([]string, error) {
	reply := make(chan []string, 1)
	ask := func(L *lua.LState) {
		var chosen []string
		if err := L.CallByParam(lua.P{Fn: cb, NRet: 1, Protect: true}, lua.LString(toWire(offered))); err == nil {
			if s, ok := L.Get(-1).(lua.LString); ok {
				chosen = wireProtocols(string(s))
			}
			L.Pop(1)
		}
		reply <- chosen
	}
	select {
	case c.calls <- ask:
	case <-c.quit:
		return nil, net.ErrClosed
	}
	select {
	case chosen := <-reply:
		return chosen, nil
	case <-c.quit:
		return nil, net.ErrClosed
	}
}

// waitFor waits for a goroutine's result as long as tm allows, a slice at
// a time so that an interrupt is noticed, running whatever Lua work the
// goroutine hands over meanwhile. It reports whether the result came.
func waitFor(L *lua.LState, tm *timeout, ch <-chan error, calls <-chan func(*lua.LState)) (error, bool) {
	for {
		retry := tm.getretry()
		if retry == 0 {
			for {
				select {
				case err := <-ch:
					return err, true
				case f := <-calls:
					f(L)
				default:
					return nil, false
				}
			}
		}
		d, last := time.Duration(pollSlice)*time.Millisecond, false
		if retry > 0 && retry*1000 <= pollSlice {
			d, last = time.Duration(retry*float64(time.Second)), true
		}
		t := time.NewTimer(d)
	wait:
		for {
			select {
			case err := <-ch:
				t.Stop()
				return err, true
			case f := <-calls:
				f(L)
			case <-t.C:
				break wait
			}
		}
		checkInterrupt(L)
		if last {
			return nil, false
		}
	}
}

func connHandshake(L *lua.LState) int {
	c := checkConn(L)
	fail := func(msg string) int {
		L.Push(lua.LFalse)
		L.Push(lua.LString(msg))
		return 2
	}
	switch {
	case c.state == sslClosed:
		return fail("closed")
	case c.state == sslConnected:
		L.Push(lua.LTrue)
		return 1
	case c.hsErr != nil:
		return fail(c.handshakeReason(c.hsErr))
	case c.nc == nil:
		if c.ncErr != nil {
			return fail(sysReason(c.ncErr))
		}
		return fail("closed")
	}
	if c.hs == nil {
		c.start()
	}
	c.tm.markstart()
	c.want = "read"
	err, done := waitFor(L, &c.tm, c.hs, c.calls)
	if !done {
		return fail("wantread")
	}
	c.want = "nothing"
	c.hs = nil
	if err != nil {
		c.hsErr = err
		return fail(c.handshakeReason(err))
	}
	c.state = sslConnected
	if c.ctx.mode == "server" && c.served == "" {
		c.served = c.tc.ConnectionState().ServerName
	}
	L.Push(lua.LTrue)
	return 1
}

// handshakeReason words a failed handshake as OpenSSL's reasons go.
func (c *sslConn) handshakeReason(err error) string {
	switch {
	case errors.Is(err, errVerify):
		return "certificate verify failed"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, net.ErrClosed),
		errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		return "closed"
	}
	msg := err.Error()
	if i := strings.LastIndex(msg, "tls: "); i >= 0 {
		msg = msg[i+len("tls: "):]
	}
	return msg
}

// code is what an error from a read or a write came to: a closed
// connection, the socket's own error, or TLS's.
func (c *sslConn) code(err error) int {
	var errno syscall.Errno
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, net.ErrClosed):
		return ioClosed
	case errors.As(err, &errno):
		return int(errno)
	}
	c.reason = c.handshakeReason(err)
	return ioSSL
}

func (c *sslConn) strerror(e int) string {
	switch e {
	case ioWantRead:
		return "wantread"
	case ioWantWrite:
		return "wantwrite"
	case ioSSL:
		return c.reason
	}
	return sockStrerror(e)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// recvInto reads what TLS has, waiting while it has nothing, a slice at a
// time; a read that times out leaves TLS as it was, to go on later.
func (c *sslConn) recvInto(L *lua.LState, p []byte, tm *timeout) (int, int) {
	if c.sticky != nil {
		err := c.sticky
		c.sticky = nil
		return 0, c.code(err)
	}
	if c.state != sslConnected {
		return 0, ioClosed
	}
	for {
		retry := tm.getretry()
		last := retry >= 0 && retry*1000 <= pollSlice
		d := time.Duration(pollSlice) * time.Millisecond
		if last {
			d = time.Duration(retry * float64(time.Second))
		}
		c.tc.SetReadDeadline(time.Now().Add(d))
		n, err := c.tc.Read(p)
		if n > 0 {
			if err != nil && !isTimeout(err) {
				c.sticky = err
			}
			return n, ioDone
		}
		switch {
		case err == nil:
		case isTimeout(err):
			checkInterrupt(L)
			if last {
				c.want = "read"
				return 0, ioWantRead
			}
		default:
			return 0, c.code(err)
		}
	}
}

// sendFrom hands data to a goroutine that writes it, one write at a time:
// written within the timeout or not, it counts as sent, and a send while
// the last one is still under way waits for it, saying "wantwrite" when it
// runs out of time.
func (c *sslConn) sendFrom(L *lua.LState, p []byte, tm *timeout) (int, int) {
	if c.state != sslConnected {
		return 0, ioClosed
	}
	if c.write != nil {
		err, done := waitFor(L, tm, c.write, nil)
		if !done {
			c.want = "write"
			return 0, ioWantWrite
		}
		c.write = nil
		if err != nil {
			return 0, c.code(err)
		}
	}
	data := append([]byte(nil), p...)
	ch := make(chan error, 1)
	tc := c.tc
	go func() {
		_, err := tc.Write(data)
		ch <- err
	}()
	err, done := waitFor(L, tm, ch, nil)
	if !done {
		c.write = ch
		return len(p), ioDone
	}
	if err != nil {
		return 0, c.code(err)
	}
	return len(p), ioDone
}

func connReceive(L *lua.LState) int {
	c := checkConn(L)
	return bufReceive(L, &c.buf, c, &c.tm, c.strerror)
}

func connSend(L *lua.LState) int {
	c := checkConn(L)
	return bufSend(L, &c.buf, c, &c.tm, c.strerror)
}

// connDirty says whether there is something to read without waiting: in
// the buffer, or decrypted, or whole records TLS has read and not yet
// handed on, which the descriptor no longer shows; a read that cannot wait
// finds those.
func connDirty(L *lua.LState) int {
	c := checkConn(L)
	dirty := false
	switch {
	case c.state == sslClosed:
	case !c.buf.isempty() || c.sticky != nil:
		dirty = true
	case c.state == sslConnected:
		c.tc.SetReadDeadline(time.Now())
		n, err := c.tc.Read(c.buf.data[:])
		c.buf.first, c.buf.last = 0, n
		if err != nil && !isTimeout(err) {
			c.sticky = err
		}
		dirty = n > 0 || c.sticky != nil
	}
	L.Push(lua.LBool(dirty))
	return 1
}

func connWant(L *lua.LState) int {
	c := checkConn(L)
	want := c.want
	if want == "" || c.state == sslClosed {
		want = "nothing"
	}
	L.Push(lua.LString(want))
	return 1
}

// connClose closes the connection, saying so to the peer, once a write
// still under way is done.
func connClose(L *lua.LState) int {
	c := checkConn(L)
	if c.state == sslClosed {
		return 0
	}
	if c.write != nil {
		select {
		case <-c.write:
		case <-time.After(5 * time.Second):
		}
		c.write = nil
	}
	close(c.quit)
	if c.state == sslConnected {
		c.tc.SetWriteDeadline(time.Now().Add(time.Second))
		c.tc.Close()
	} else if c.nc != nil {
		c.nc.Close()
	}
	c.state = sslClosed
	c.fd = sockInvalid
	return 0
}

func connected(L *lua.LState, c *sslConn) bool {
	if c.state != sslConnected {
		L.Push(lua.LNil)
		L.Push(lua.LString("closed"))
		return false
	}
	return true
}

func connPeerCert(L *lua.LState) int {
	c := checkConn(L)
	if !connected(L, c) {
		return 2
	}
	n := L.OptInt(2, 1)
	if n < 1 {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid certificate index"))
		return 2
	}
	peers := c.tc.ConnectionState().PeerCertificates
	if n > len(peers) {
		L.Push(lua.LNil)
		return 1
	}
	pushCert(L, peers[n-1])
	return 1
}

func connPeerChain(L *lua.LState) int {
	c := checkConn(L)
	if !connected(L, c) {
		return 2
	}
	t := L.NewTable()
	for _, cert := range c.tc.ConnectionState().PeerCertificates {
		pushCert(L, cert)
		t.Append(L.Get(-1))
		L.Pop(1)
	}
	L.Push(t)
	return 1
}

// localChain is the certificates this side has to show.
func (c *sslConn) localChain() []*x509.Certificate {
	if len(c.ctx.certs) == 0 {
		return nil
	}
	var out []*x509.Certificate
	for _, der := range c.ctx.certs[0].Certificate {
		if cert, err := x509.ParseCertificate(der); err == nil {
			out = append(out, cert)
		}
	}
	return out
}

func connLocalCert(L *lua.LState) int {
	c := checkConn(L)
	if !connected(L, c) {
		return 2
	}
	n := L.OptInt(2, 1)
	if n < 1 {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid certificate index"))
		return 2
	}
	chain := c.localChain()
	if n > len(chain) {
		L.Push(lua.LNil)
		return 1
	}
	pushCert(L, chain[n-1])
	return 1
}

func connLocalChain(L *lua.LState) int {
	c := checkConn(L)
	if !connected(L, c) {
		return 2
	}
	t := L.NewTable()
	for _, cert := range c.localChain() {
		pushCert(L, cert)
		t.Append(L.Get(-1))
		L.Pop(1)
	}
	L.Push(t)
	return 1
}

func connPeerVerification(L *lua.LState) int {
	c := checkConn(L)
	if c.state != sslConnected {
		L.Push(lua.LFalse)
		L.Push(lua.LString("closed"))
		return 2
	}
	if c.verify == "" {
		L.Push(lua.LTrue)
		return 1
	}
	L.Push(lua.LFalse)
	L.Push(lua.LString(c.verify))
	return 2
}

func connGetALPN(L *lua.LState) int {
	c := checkConn(L)
	if c.state != sslConnected || c.tc.ConnectionState().NegotiatedProtocol == "" {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(c.tc.ConnectionState().NegotiatedProtocol))
	return 1
}

func connGetSNIName(L *lua.LState) int {
	c := checkConn(L)
	name := c.sni
	if c.ctx.mode == "server" {
		name = c.served
	}
	if name == "" {
		L.Push(lua.LNil)
	} else {
		L.Push(lua.LString(name))
	}
	return 1
}

func connExportKeyingMaterial(L *lua.LState) int {
	c := checkConn(L)
	if !connected(L, c) {
		return 0
	}
	label := L.CheckString(2)
	n := L.CheckInt(3)
	var context []byte
	if s, ok := L.Get(4).(lua.LString); ok {
		context = []byte(s)
	}
	cs := c.tc.ConnectionState()
	out, err := cs.ExportKeyingMaterial(label, context, n)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("error exporting keying material"))
		return 2
	}
	L.Push(lua.LString(out))
	return 1
}

// connCompression is never anything: TLS compression is long gone.
func connCompression(L *lua.LState) int {
	c := checkConn(L)
	if !connected(L, c) {
		return 2
	}
	L.Push(lua.LNil)
	return 1
}

// cipherInfo is a cipher suite as OpenSSL describes it, for info().
type cipherInfo struct {
	name, proto, kx, au, enc, mac string
	bits                          int
}

var cipherInfos = map[uint16]cipherInfo{
	tls.TLS_AES_128_GCM_SHA256:                        {"TLS_AES_128_GCM_SHA256", "TLSv1.3", "any", "any", "AESGCM(128)", "AEAD", 128},
	tls.TLS_AES_256_GCM_SHA384:                        {"TLS_AES_256_GCM_SHA384", "TLSv1.3", "any", "any", "AESGCM(256)", "AEAD", 256},
	tls.TLS_CHACHA20_POLY1305_SHA256:                  {"TLS_CHACHA20_POLY1305_SHA256", "TLSv1.3", "any", "any", "CHACHA20/POLY1305(256)", "AEAD", 256},
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:         {"ECDHE-RSA-AES128-GCM-SHA256", "TLSv1.2", "ECDH", "RSA", "AESGCM(128)", "AEAD", 128},
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:       {"ECDHE-ECDSA-AES128-GCM-SHA256", "TLSv1.2", "ECDH", "ECDSA", "AESGCM(128)", "AEAD", 128},
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:         {"ECDHE-RSA-AES256-GCM-SHA384", "TLSv1.2", "ECDH", "RSA", "AESGCM(256)", "AEAD", 256},
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:       {"ECDHE-ECDSA-AES256-GCM-SHA384", "TLSv1.2", "ECDH", "ECDSA", "AESGCM(256)", "AEAD", 256},
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256:   {"ECDHE-RSA-CHACHA20-POLY1305", "TLSv1.2", "ECDH", "RSA", "CHACHA20/POLY1305(256)", "AEAD", 256},
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256: {"ECDHE-ECDSA-CHACHA20-POLY1305", "TLSv1.2", "ECDH", "ECDSA", "CHACHA20/POLY1305(256)", "AEAD", 256},
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:            {"ECDHE-RSA-AES128-SHA", "SSLv3", "ECDH", "RSA", "AES(128)", "SHA1", 128},
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:          {"ECDHE-ECDSA-AES128-SHA", "SSLv3", "ECDH", "ECDSA", "AES(128)", "SHA1", 128},
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:            {"ECDHE-RSA-AES256-SHA", "SSLv3", "ECDH", "RSA", "AES(256)", "SHA1", 256},
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:          {"ECDHE-ECDSA-AES256-SHA", "SSLv3", "ECDH", "ECDSA", "AES(256)", "SHA1", 256},
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256:               {"AES128-GCM-SHA256", "TLSv1.2", "RSA", "RSA", "AESGCM(128)", "AEAD", 128},
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384:               {"AES256-GCM-SHA384", "TLSv1.2", "RSA", "RSA", "AESGCM(256)", "AEAD", 256},
	tls.TLS_RSA_WITH_AES_128_CBC_SHA:                  {"AES128-SHA", "SSLv3", "RSA", "RSA", "AES(128)", "SHA1", 128},
	tls.TLS_RSA_WITH_AES_256_CBC_SHA:                  {"AES256-SHA", "SSLv3", "RSA", "RSA", "AES(256)", "SHA1", 256},
}

var versionNames = map[uint16]string{
	tls.VersionTLS10: "TLSv1", tls.VersionTLS11: "TLSv1.1",
	tls.VersionTLS12: "TLSv1.2", tls.VersionTLS13: "TLSv1.3",
}

// connInfo is core.info(conn): the cipher as OpenSSL describes it, its
// bits twice, and the protocol, which ssl.lua's info() reads.
func connInfo(L *lua.LState) int {
	c := checkConn(L)
	if c.state != sslConnected {
		return 0
	}
	cs := c.tc.ConnectionState()
	ci, ok := cipherInfos[cs.CipherSuite]
	if !ok {
		ci = cipherInfo{tls.CipherSuiteName(cs.CipherSuite), versionNames[cs.Version], "unknown", "unknown", "unknown", "unknown", 0}
	}
	L.Push(lua.LString(fmt.Sprintf("%-23s %s Kx=%-8s Au=%-4s Enc=%s Mac=%s\n", ci.name, ci.proto, ci.kx, ci.au, ci.enc, ci.mac)))
	L.Push(lua.LNumber(ci.bits))
	L.Push(lua.LNumber(ci.bits))
	L.Push(lua.LString(versionNames[cs.Version]))
	return 4
}

// sysReason is a system error without Go's wrapping: "no such file or
// directory", as OpenSSL's reasons read.
func sysReason(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

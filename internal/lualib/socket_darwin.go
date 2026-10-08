package lualib

// macOS has none of the TCP options that only Linux has.
func tcpExtraOpts() map[string]optPair { return nil }

// disconnect does nothing on macOS, which refuses an AF_UNSPEC connect
// with EAFNOSUPPORT; LuaSocket ignores that error too.
func disconnect(fd, family int) {}

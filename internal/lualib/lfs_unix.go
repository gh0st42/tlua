//go:build unix

package lualib

import (
	"os"

	"golang.org/x/sys/unix"

	lua "github.com/yuin/gopher-lua"
)

// fileAttrs is everything stat(2) said, under lfs's names. It asks again
// through x/sys/unix, whose Stat_t spells its fields the same way on every
// Unix, which syscall's does not.
func fileAttrs(path string, fi os.FileInfo, link bool) []attr {
	var st unix.Stat_t
	stat := unix.Stat
	if link {
		stat = unix.Lstat
	}
	if stat(path, &st) != nil {
		return portableAttrs(fi)
	}
	return []attr{
		{"dev", lua.LNumber(st.Dev)},
		{"ino", lua.LNumber(st.Ino)},
		{"mode", lua.LString(modeName(fi.Mode()))},
		{"nlink", lua.LNumber(st.Nlink)},
		{"uid", lua.LNumber(st.Uid)},
		{"gid", lua.LNumber(st.Gid)},
		{"rdev", lua.LNumber(st.Rdev)},
		{"access", lua.LNumber(st.Atim.Sec)},
		{"modification", lua.LNumber(st.Mtim.Sec)},
		{"change", lua.LNumber(st.Ctim.Sec)},
		{"size", lua.LNumber(st.Size)},
		{"permissions", lua.LString(permString(fi.Mode()))},
		{"blocks", lua.LNumber(st.Blocks)},
		{"blksize", lua.LNumber(st.Blksize)},
	}
}

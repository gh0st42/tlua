//go:build !unix

package lualib

import "os"

// fileAttrs is what can be said about a file without stat(2).
func fileAttrs(path string, fi os.FileInfo, link bool) []attr {
	return portableAttrs(fi)
}

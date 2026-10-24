//go:build !unix

package guilock

// Acquire does nothing where there is no file locking to use.
func Acquire() {}

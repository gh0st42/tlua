// Package version is tlua's version number, in one place so that what the
// interpreter prints and what it tells a language server cannot drift apart.
package version

// Number is the release, as major.minor.patch.
const Number = "0.6.2"

// Build is the platform a release build was made for, as its archive is
// named (tlua-VERSION-<Build>.tar.gz): scripts/build-release.sh sets it with
// -ldflags "-X tlua/internal/version.Build=...". A tlua built any other way
// leaves it empty, which is how tlua update knows not to replace it.
var Build string

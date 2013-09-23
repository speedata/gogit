// Read access to a git repository.
//
// The api tries to be compatible with the Go bindings of libgit2 (git2go). Some methods
// seem to be unnecessary (such as Tree.EntryByIndex()), this is because they exist in
// git2go, but are not strictly necessary in a pure Go implementation.
package gogit

// Version information: ‹Major›.‹Minor›.‹Patchlevel›
const (
	VersionMajor      = 0
	VersionMinor      = 0
	VersionPatchlevel = 0
)

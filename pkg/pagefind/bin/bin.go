package bin

var embeddedGz []byte

// GetEmbeddedPagefindGz returns the compressed Pagefind binary for the current target OS/Arch,
// or nil if the platform is unsupported.
func GetEmbeddedPagefindGz() []byte {
	return embeddedGz
}

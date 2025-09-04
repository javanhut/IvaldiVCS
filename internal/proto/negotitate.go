package proto

// NegotiateCompression picks the best common compression between local preference
// (preferZstd) and the remote's advertised capabilities.
//
// remoteCaps example: []string{"zlib","zstd"}
// return value: "zstd" or "zlib"
func NegotiateCompression(remoteCaps []string, preferZstd bool) string {
	hasZstd := false
	hasZlib := false
	for _, c := range remoteCaps {
		switch c {
		case "zstd":
			hasZstd = true
		case "zlib", "deflate":
			hasZlib = true
		}
	}
	if preferZstd && hasZstd {
		return "zstd"
	}
	// fallback
	if hasZlib {
		return "zlib"
	}
	// last resort default
	if hasZstd {
		return "zstd"
	}
	return "zlib"
}

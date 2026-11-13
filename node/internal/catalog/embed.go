package catalog

import _ "embed"

// Embedded is the catalog built into this binary, and its signature.
var (
	//go:embed embedded/catalog.json
	Embedded []byte
	//go:embed embedded/catalog.json.sig
	EmbeddedSig []byte
)

// Package release holds the Outpost release signing key and default URLs.
package release

// PublicKey verifies release checksums (namespace "outpost-release") and the
// content catalog (namespace "outpost-catalog").
const PublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBrMpCCBsGNt2QYPvv84kSXqzkIMJIvTJAgadKprEcLV outpost-release"

// SiteURL is the Outpost website, which hosts the catalog and releases.
const SiteURL = "https://outpost-offline.vercel.app"

// CatalogURL is where the signed catalog is published.
const CatalogURL = SiteURL + "/catalog/v1/catalog.json"

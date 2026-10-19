// Package providers embeds the manifests in this directory into the pobear
// binary, so a provider like Decodo works out of the box without the user
// placing any files on disk. See docs/PROVIDER_REFERENCE.md §13.
package providers

import "embed"

//go:generate go run ./internal/decodogen

//go:embed *.yaml
var FS embed.FS

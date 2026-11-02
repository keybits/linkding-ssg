//go:build darwin && amd64

package bin

import (
	_ "embed"
)

//go:embed pagefind_darwin_amd64.gz
var embeddedPagefindGz []byte

func init() {
	embeddedGz = embeddedPagefindGz
}

//go:build darwin && arm64

package bin

import (
	_ "embed"
)

//go:embed pagefind_darwin_arm64.gz
var embeddedPagefindGz []byte

func init() {
	embeddedGz = embeddedPagefindGz
}

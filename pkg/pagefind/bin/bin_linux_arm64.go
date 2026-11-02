//go:build linux && arm64

package bin

import (
	_ "embed"
)

//go:embed pagefind_linux_arm64.gz
var embeddedPagefindGz []byte

func init() {
	embeddedGz = embeddedPagefindGz
}

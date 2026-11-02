//go:build linux && amd64

package bin

import (
	_ "embed"
)

//go:embed pagefind_linux_amd64.gz
var embeddedPagefindGz []byte

func init() {
	embeddedGz = embeddedPagefindGz
}

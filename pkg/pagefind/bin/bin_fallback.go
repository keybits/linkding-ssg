//go:build !( (darwin && arm64) || (darwin && amd64) || (linux && amd64) || (linux && arm64) )

package bin

func init() {
	embeddedGz = nil
}

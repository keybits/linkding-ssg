package pagefind

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// IndexSite runs pagefind against siteDir.
// It searches for `pagefind` binary in PATH first; if not found, it runs via `npx -y pagefind`.
func IndexSite(ctx context.Context, siteDir string) error {
	var cmd *exec.Cmd

	if pagefindPath, err := exec.LookPath("pagefind"); err == nil {
		cmd = exec.CommandContext(ctx, pagefindPath, "--site", siteDir)
	} else if npxPath, err := exec.LookPath("npx"); err == nil {
		cmd = exec.CommandContext(ctx, npxPath, "-y", "pagefind", "--site", siteDir)
	} else {
		return fmt.Errorf("neither 'pagefind' nor 'npx' was found in PATH. Please install pagefind or Node.js/npx")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pagefind indexing failed: %w", err)
	}

	return nil
}

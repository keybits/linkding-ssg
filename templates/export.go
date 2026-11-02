package templates

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ExportAssets exports embedded HTML templates and static assets to a target directory.
// Typically targetDir is "data/templates" and "data/static", or a parent like "data".
func ExportAssets(targetDir string) error {
	if targetDir == "" {
		targetDir = "data"
	}

	// 1. Export html templates to targetDir/templates
	htmlTarget := filepath.Join(targetDir, "templates")
	if err := exportSubFS("html", htmlTarget); err != nil {
		return fmt.Errorf("exporting templates: %w", err)
	}

	// 2. Export static assets to targetDir/static
	staticTarget := filepath.Join(targetDir, "static")
	if err := exportSubFS("static", staticTarget); err != nil {
		return fmt.Errorf("exporting static assets: %w", err)
	}

	return nil
}

func exportSubFS(subDirName, destDir string) error {
	sub, err := fs.Sub(EmbeddedFS, subDirName)
	if err != nil {
		return fmt.Errorf("accessing embedded subFS %s: %w", subDirName, err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", destDir, err)
	}

	return fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		targetPath := filepath.Join(destDir, path)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		srcFile, err := sub.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

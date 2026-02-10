package skills

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyStatus describes the outcome of a copy attempt.
type CopyStatus string

const (
	CopyStatusCopied   CopyStatus = "copied"
	CopyStatusConflict CopyStatus = "conflict"
)

// CopyResult wraps the final status for a copy operation.
type CopyResult struct {
	Status CopyStatus
}

// Copy recursively copies sourcePath into destPath when destPath does not exist.
func Copy(sourcePath, destPath string) (CopyResult, error) {
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return CopyResult{}, err
	}

	sourceInfo, err := os.Stat(sourceAbs)
	if err != nil {
		return CopyResult{}, fmt.Errorf("source missing %q: %w", sourceAbs, err)
	}
	if !sourceInfo.IsDir() {
		return CopyResult{}, fmt.Errorf("source is not a directory %q", sourceAbs)
	}

	if _, err := os.Lstat(destPath); err == nil {
		return CopyResult{Status: CopyStatusConflict}, nil
	} else if !os.IsNotExist(err) {
		return CopyResult{}, err
	}

	parent := filepath.Dir(destPath)
	tmpDir, err := os.MkdirTemp(parent, "."+filepath.Base(destPath)+".tmp-*")
	if err != nil {
		return CopyResult{}, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := os.Chmod(tmpDir, sourceInfo.Mode().Perm()); err != nil {
		return CopyResult{}, err
	}

	if err := filepath.WalkDir(sourceAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(sourceAbs, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		destEntry := filepath.Join(tmpDir, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}

		mode := info.Mode()
		switch {
		case mode.IsDir():
			return os.Mkdir(destEntry, mode.Perm())
		case mode.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, destEntry)
		case mode.IsRegular():
			return copyFile(path, destEntry, mode.Perm())
		default:
			return fmt.Errorf("unsupported file type %q", path)
		}
	}); err != nil {
		return CopyResult{}, err
	}

	if err := os.Rename(tmpDir, destPath); err != nil {
		if _, statErr := os.Lstat(destPath); statErr == nil {
			return CopyResult{Status: CopyStatusConflict}, nil
		}
		return CopyResult{}, err
	}

	success = true
	return CopyResult{Status: CopyStatusCopied}, nil
}

func copyFile(sourcePath, destPath string, mode fs.FileMode) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		_ = sourceFile.Close()
		return err
	}

	_, copyErr := io.Copy(destFile, sourceFile)
	destCloseErr := destFile.Close()
	sourceCloseErr := sourceFile.Close()

	if copyErr != nil {
		return copyErr
	}
	if destCloseErr != nil {
		return destCloseErr
	}
	if sourceCloseErr != nil {
		return sourceCloseErr
	}
	if err := os.Chmod(destPath, mode); err != nil {
		return err
	}
	return nil
}

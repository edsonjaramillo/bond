package application

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type copiedDirectory struct {
	path string
	mode os.FileMode
}

func copySkill(source, destination string) error {
	var directories []copiedDirectory
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("inspect source entry %q: %w", relativeTreePath(source, path), walkErr)
		}
		relative := relativeTreePath(source, path)
		target := destination
		if path != source {
			target = filepath.Join(destination, filepath.FromSlash(relative))
		}
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect source entry %q: %w", relative, err)
		}

		switch {
		case info.IsDir():
			if err := os.Mkdir(target, info.Mode().Perm()|0o700); err != nil {
				return fmt.Errorf("create copied directory %q: %w", relative, err)
			}
			directories = append(directories, copiedDirectory{path: target, mode: info.Mode().Perm()})

			return nil
		case info.Mode().IsRegular():
			if err := copyRegularFile(path, target, info.Mode().Perm()); err != nil {
				return fmt.Errorf("copy regular file %q: %w", relative, err)
			}

			return nil
		case info.Mode()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read source symlink %q: %w", relative, err)
			}
			if filepath.IsAbs(linkTarget) {
				return fmt.Errorf("source symlink %q must have a relative target", relative)
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				return fmt.Errorf("copy symlink %q: %w", relative, err)
			}

			return nil
		default:
			return fmt.Errorf("source entry %q must be a regular file, directory, or safe relative symlink", relative)
		}
	})
	if err != nil {
		return err
	}

	for index := len(directories) - 1; index >= 0; index-- {
		directory := directories[index]
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return fmt.Errorf("set copied directory permissions %q: %w", relativeTreePath(destination, directory.path), err)
		}
		if err := syncDirectory(directory.path); err != nil {
			return err
		}
	}

	return nil
}

func copyRegularFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		_ = input.Close()

		return err
	}
	removeOutput := true
	defer func() {
		_ = output.Close()
		if removeOutput {
			_ = os.Remove(destination)
		}
	}()
	_, copyErr := io.Copy(output, input)
	closeInputErr := input.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeInputErr != nil {
		return closeInputErr
	}
	if err := output.Chmod(mode); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	removeOutput = false

	return nil
}

type skillFingerprint string

func (fingerprint skillFingerprint) valid() bool {
	decoded, err := hex.DecodeString(string(fingerprint))

	return err == nil && len(decoded) == sha256.Size
}

func skillTreeFingerprint(root string) (skillFingerprint, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		relative := relativeTreePath(root, path)
		if _, err := fmt.Fprintf(hash, "%s\x00%o\x00", relative, info.Mode()); err != nil {
			return err
		}
		switch {
		case info.Mode().IsRegular():
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(hash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if _, err := io.WriteString(hash, target); err != nil {
				return err
			}
		case info.IsDir():
		default:
			return fmt.Errorf("entry %q has unsupported type", relative)
		}
		_, err = hash.Write([]byte{0})

		return err
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint Project Skill: %w", err)
	}

	return skillFingerprint(hex.EncodeToString(hash.Sum(nil))), nil
}

func relativeTreePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}

	return filepath.ToSlash(relative)
}

type filesystemIdentity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func identifyPath(path string) (filesystemIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return filesystemIdentity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return filesystemIdentity{}, fmt.Errorf("inspect filesystem identity for %q", path)
	}

	return filesystemIdentity{Device: uint64(stat.Dev), Inode: uint64(stat.Ino)}, nil
}

func (identity filesystemIdentity) recorded() bool {
	return identity.Device != 0 || identity.Inode != 0
}

func removeInstalledPath(path string, mode installationMode) error {
	if mode == copyMode {
		return os.RemoveAll(path)
	}

	return os.Remove(path)
}

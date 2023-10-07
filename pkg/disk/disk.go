package disk

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func EnsureDirectoryExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// RemoveIfExists attempts to remove the given named file or (empty) directory,
// ignoring IsNotExist errors.
func RemoveIfExists(filename string) error {
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ForceRemove attempts to delete a directory using os.RemoveAll. If that fails,
// it will attempt to traverse the directory and update permissions so that the
// directory can be removed, then retry os.RemoveAll. This fallback approach is
// used for performance reasons, since recursive chmod can be slow for very
// large directories.
func ForceRemove(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	err = filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// In order to remove dir entries and make sure we can further
			// recurse into the dir, we need all bits set (RWX).
			return os.Chmod(path, 0770)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func FileExists(fullPath string) (bool, error) {
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func HeadLink(oldname, newname string) error {
	if err := os.Link(oldname, newname); err != nil {
		return fmt.Errorf("failed to link %q to %q: %w", oldname, newname, err)
	}

	return nil
}

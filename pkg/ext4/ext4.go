package ext4

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// https://github.com/buildbuddy-io/buildbuddy/blob/master/enterprise/server/util/ext4/ext4.go

// DirectoryToImageAutoSize is like DirectoryToImage, but it will attempt to
// automatically pick a file size that is "big enough".
func DirectoryToImageAutoSize(ctx context.Context, inputDir, outputFile string) error {
	dirSizeBytes, err := DiskSizeBytes(ctx, inputDir)
	if err != nil {
		return err
	}

	//nolint:gomnd // this why
	imageSizeBytes := int64(float64(dirSizeBytes)*1.2) + 1000000
	return DirectoryToImage(ctx, inputDir, outputFile, imageSizeBytes)
}

// DiskSizeBytes returns the size in bytes of a directory according to "du -sk".
// It can be used when creating ext4 images -- to ensure they are large enough.
func DiskSizeBytes(ctx context.Context, inputDir string) (int64, error) {
	out, err := exec.CommandContext(ctx, "du", "-sk", inputDir).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to run 'du -sk': %w: %s", err, out)
	}

	parts := strings.Split(string(out), "\t")
	//nolint:gomnd // this why
	if len(parts) != 2 {
		return 0, fmt.Errorf("du output %q did not match 'SIZE /file/path'", out)
	}
	s, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("du output %q did not match 'SIZE /file/path': %w", inputDir, err)
	}
	//nolint:gomnd // this why
	return int64(s * 1000), nil
}

// DirectoryToImage creates an ext4 image of the specified size from inputDir
// and writes it to outputFile.
// https://linux.die.net/man/8/mke2fs
func DirectoryToImage(ctx context.Context, inputDir, outputFile string, sizeBytes int64) error {
	if err := checkImageOutputPath(outputFile); err != nil {
		return err
	}

	args := []string{
		"/sbin/mke2fs",
		"-L", "''",
		"-N", "0",
		// "-O", "^64bit",
		"-d", inputDir,
		"-m", "5",
		"-r", "1",
		// "-t", "ext4",
		outputFile,
		fmt.Sprintf("%dK", sizeBytes/1e3),
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run'mke2fs': %w: %s", err, cmd.String())
	}
	return nil
}

// Checks an image output path to make sure a non-empty file doesn't already
// exist at that path. Overwriting an existing image can cause corruption.
func checkImageOutputPath(path string) error {
	stat, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat output path: %s, error: %w", path, err)
	}
	if stat != nil && stat.Size() > 0 {
		return fmt.Errorf("failed to stat output path: %s, file already exists", path)
	}
	return nil
}

// MakeEmptyImage creates a new empty ext4 disk image of the specified size
// and writes it to outputFile.
func MakeEmptyImage(ctx context.Context, outputFile string, sizeBytes int64) error {
	if err := checkImageOutputPath(outputFile); err != nil {
		return err
	}

	args := []string{
		"/sbin/mke2fs",
		"-L", "",
		"-N", "0",
		"-O", "^64bit",
		"-m", "5",
		"-r", "1",
		"-t", "ext4",
		outputFile,
		fmt.Sprintf("%dK", sizeBytes/1e3),
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run 'mke2fs': %w: %s", err, out)
	}
	return nil
}

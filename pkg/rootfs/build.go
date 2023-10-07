package rootfs

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/go-logr/logr"
	"golang.org/x/sync/singleflight"

	"github.com/koolay/buildfs/pkg/disk"
	"github.com/koolay/buildfs/pkg/ext4"
	"github.com/koolay/buildfs/pkg/str"
)

const (
	diskImageFileName = "containerfs.ext4"

	// Minimum timeout used for background Firecracker disk image conversion.
	imageConversionTimeout = 15 * time.Minute
)

var (
	isRoot bool

	// Single-flight group used to dedupe firecracker image conversions.
	conversionGroup singleflight.Group
)

func init() {
	u, err := user.Current()
	if err != nil {
		log.Printf("could not determine current user: %v\n", err)
	} else {
		isRoot = u.Uid == "0"
	}
}

type PullCredentials struct {
	Username string
	Password string
}

func (p PullCredentials) IsEmpty() bool {
	return p.Username == "" && p.Password == ""
}

func (p PullCredentials) String() string {
	if p.Username == "" && p.Password == "" {
		return ""
	}

	return p.Username + ":" + p.Password
}

func (p PullCredentials) ToRegistryAuth() string {
	if p.Username == "" && p.Password == "" {
		return ""
	}

	authCfg := dockertypes.AuthConfig{
		Username: p.Username,
		Password: p.Password,
	}

	buf, _ := json.Marshal(authCfg)
	return base64.URLEncoding.EncodeToString(buf)
}

type Builder struct {
	logger *logr.Logger
	puller *ImagePuller
}

func NewBuilder(logger *logr.Logger) *Builder {
	return &Builder{logger: logger, puller: NewImagePuller(logger)}
}

/*
CreateDiskImage creates a disk image for a container image in the specified workspace directory.
It first checks if a cached disk image already exists for the given container image and workspace directory.
If a cached disk image exists, it returns the path to the cached image.
Otherwise, it deduplicates image conversion operations, which are disk IO-heavy,
and converts the image in the background.
The function applies a timeout to the background conversion to prevent it from running forever.
If the context is cancelled before the conversion is complete, the function returns an error.

Parameters:

  - workspaceDir (string): The path to the workspace directory where the disk image will be created.

  - containerImage (string): The name of the container image for which the disk image will be created,
    like: daocloud.io/library/nginx/alpine:1.12.0-alpine.

  - creds (PullCredentials): The credentials required to pull the container image.

Returns:
- string: The path to the created disk image.
- error: An error if the function encounters any issues during execution.

Errors:
- If the function encounters an error while checking for a cached disk image, it returns an error.
- If the function encounters
*/
func (r *Builder) CreateDiskImage(
	ctx context.Context,
	workspaceDir,
	containerImage string,
	creds PullCredentials,
) (string, error) {
	existingPath, err := r.cachedDiskImagePath(ctx, workspaceDir, containerImage)
	if err != nil {
		return "", err
	}

	if existingPath != "" {
		return existingPath, nil
	}

	conversionOpKey := singleflightKey(
		workspaceDir, containerImage, creds.Username, creds.Password,
	)
	resultChan := conversionGroup.DoChan(conversionOpKey, func() (interface{}, error) {
		sctx, cancel := context.WithTimeout(context.Background(), imageConversionTimeout)
		defer cancel()
		// NOTE: If more params are added to this func, be sure to update
		// conversionOpKey above (if applicable).
		return r.createExt4Image(sctx, workspaceDir, containerImage)
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultChan:
		if res.Err != nil {
			return "", res.Err
		}
		if res.Shared {
			r.logger.Info("duplicated firecracker disk image conversion", "image", containerImage)
		}
		return res.Val.(string), nil
	}
}

func (r *Builder) getLocalImagePath(workspaceDir, containerImage string) string {
	hashedContainerName := str.HashString(containerImage)
	return filepath.Join(workspaceDir, "containers", hashedContainerName)
}

// cachedDiskImagePath looks for an existing cached disk image and returns the
// path to it, if it exists. It returns "" (with no error) if the disk image
// does not exist and no other errors occurred while looking for the image.
func (r *Builder) cachedDiskImagePath(
	ctx context.Context,
	workspaceDir, containerImage string,
) (string, error) {
	containerImagesPath := r.getLocalImagePath(workspaceDir, containerImage)
	files, err := os.ReadDir(containerImagesPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	sort.Slice(files, func(i, j int) bool {
		var iUnix int64
		if fi, serr := files[i].Info(); serr == nil {
			iUnix = fi.ModTime().Unix()
		}
		var jUnix int64
		if fi, serr := files[j].Info(); serr == nil {
			jUnix = fi.ModTime().Unix()
		}
		return iUnix < jUnix
	})
	diskImagePath := filepath.Join(
		containerImagesPath,
		files[len(files)-1].Name(),
		diskImageFileName,
	)
	r.logger.Info("check image cache", "path", diskImagePath)
	exists, err := disk.FileExists(diskImagePath)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	return diskImagePath, nil
}

func (r *Builder) createExt4Image(
	ctx context.Context,
	workspaceDir, containerImage string,
) (string, error) {
	containerImagesPath := r.getLocalImagePath(workspaceDir, containerImage)

	tmpImagePath, err := r.pullContainerToExt4FS(ctx, containerImage, workspaceDir)
	if err != nil {
		return "", err
	}

	imageHash, err := r.hashFile(tmpImagePath)
	if err != nil {
		return "", err
	}
	containerImageHome := filepath.Join(containerImagesPath, imageHash)
	r.logger.Info("pulled image", "path", tmpImagePath, "rootfs-path", containerImageHome)
	if serr := disk.EnsureDirectoryExists(containerImageHome); serr != nil {
		return "", serr
	}
	containerImagePath := filepath.Join(containerImageHome, diskImageFileName)
	if serr := os.Rename(tmpImagePath, containerImagePath); serr != nil {
		return "", serr
	}
	return containerImagePath, nil
}

// Pull pull docker image and to ext4 fs
// After running these commands, /tmp/image_unpack/bundle/rootfs/ has the
// unpacked image contents.
// umoci unpack --rootless --image /tmp/image_unpack/oci_image /tmp/image_unpack/bundle
// https://github.com/buildbuddy-io/buildbuddy/blob/
// f09e84c5c3c96eb5f670823a09ac899ec4d44ec4/enterprise/server/util/container/container.go#L200
func (r *Builder) pullContainerToExt4FS(
	ctx context.Context,
	srcImage string,
	workspaceDir string,
) (string, error) {
	r.logger.Info("pull image", "src", srcImage)
	var rootUnpackDir string
	// Make a temp directory to work in. Delete it when this fuction returns.
	rootUnpackDir, err := os.MkdirTemp(workspaceDir, "container-unpack-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(rootUnpackDir)

	// Make a directory to download the OCI image to.
	ociImageDir := filepath.Join(rootUnpackDir, "image")
	if serr := disk.EnsureDirectoryExists(ociImageDir); serr != nil {
		return "", fmt.Errorf("failed to create directory: %s: %w", ociImageDir, serr)
	}

	// oci:/tmp/skopeo/container-unpack-1665441197/image:latest
	ociOutputRef := fmt.Sprintf("oci:%s:latest", ociImageDir)
	err = r.puller.Pull(
		ctx,
		PullOptions{SrcImage: srcImage, DestImage: ociOutputRef, OS: "linux"},
		os.Stdout,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to pull image, src: %s, dest: %s, error: %w",
			srcImage,
			ociOutputRef,
			err,
		)
	}

	r.logger.Info("Unpacking OCI image", "image", srcImage)
	// Make a directory to unpack the bundle to.
	// /tmp/skopeo/container-unpack-1665441197/rootfs
	rootFSDir := filepath.Join(rootUnpackDir, "rootfs")
	if serr := disk.EnsureDirectoryExists(rootFSDir); serr != nil {
		return "", fmt.Errorf("failed to create rootfs directory: %s: %w", rootFSDir, serr)
	}

	err = unrawpack(!isRoot, ociImageDir, rootFSDir)
	if err != nil {
		return "", fmt.Errorf("failed to unpack OCI image: %w", err)
	}

	// Take the rootfs and write it into an ext4 image.
	f, err := os.CreateTemp(workspaceDir, "containerfs-*.ext4")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %s: %w", workspaceDir, err)
	}

	defer f.Close()
	imageFile := f.Name()
	if serr := ext4.DirectoryToImageAutoSize(ctx, rootFSDir, imageFile); serr != nil {
		return "", serr
	}
	return imageFile, nil
}

func (r *Builder) hashFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, serr := io.Copy(h, f); serr != nil {
		return "", serr
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// singleflightKey returns a key that can be used to dedupe a function whose
// output depends solely on the given args.
func singleflightKey(args ...string) string {
	h := ""
	for _, s := range args {
		h += str.HashString(s)
	}
	return str.HashString(h)
}

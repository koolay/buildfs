package rootfs

import (
	"context"
	"fmt"
	"os"
	"strings"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/opencontainers/umoci/pkg/idtools"
	"github.com/pkg/errors"
)

// https://github.com/opencontainers/umoci/blob/fb2db51251ac2cb3745e60b2f2f314a088400326/utils.go#L290
func parseIdmapOptions(meta *umoci.Meta, rootless bool) error {
	// We need to set mappings if we're in rootless mode.
	meta.MapOptions.Rootless = rootless
	if meta.MapOptions.Rootless {
		uidmap := fmt.Sprintf("0:%d:1", os.Geteuid())
		gidmap := fmt.Sprintf("0:%d:1", os.Getegid())

		idMap, err := idtools.ParseMapping(uidmap)
		if err != nil {
			return errors.Wrapf(err, "failure parsing --uid-map %s", uidmap)
		}
		meta.MapOptions.UIDMappings = append(meta.MapOptions.UIDMappings, idMap)

		idMap2, err := idtools.ParseMapping(gidmap)
		if err != nil {
			return errors.Wrapf(err, "failure parsing --gid-map %s", gidmap)
		}
		meta.MapOptions.GIDMappings = append(meta.MapOptions.GIDMappings, idMap2)
	}

	return nil
}

// https://github.com/opencontainers/umoci/blob/main/cmd/umoci/raw-unpack.go
func unrawpack(rootless bool, image string, rootfsPath string) error {
	var (
		imagePath string
		tag       string
	)
	sep := strings.Index(image, ":")
	if sep == -1 {
		imagePath = image
		tag = "latest"
	} else {
		imagePath = image[:sep]
		tag = image[sep+1:]
	}

	var unpackOptions layer.UnpackOptions
	var meta umoci.Meta
	meta.Version = umoci.MetaVersion
	// Parse and set up the mapping options.
	err := parseIdmapOptions(&meta, rootless)
	if err != nil {
		return err
	}

	unpackOptions.MapOptions = meta.MapOptions

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	fromDescriptorPaths, err := engineExt.ResolveReference(context.Background(), tag)
	if err != nil {
		return errors.Wrap(err, "get descriptor")
	}
	if len(fromDescriptorPaths) == 0 {
		return errors.Errorf("tag is not found: %s", tag)
	}
	if len(fromDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return errors.Errorf("tag is ambiguous: %s", tag)
	}
	meta.From = fromDescriptorPaths[0]

	manifestBlob, err := engineExt.FromDescriptor(context.Background(), meta.From.Descriptor())
	if err != nil {
		return errors.Wrap(err, "get manifest")
	}
	defer manifestBlob.Close()

	if manifestBlob.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return errors.Wrap(
			fmt.Errorf(
				"descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s",
				manifestBlob.Descriptor.MediaType,
			),
			"invalid --image tag",
		)
	}

	// Get the manifest.
	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return errors.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.Descriptor.MediaType)
	}

	if err := layer.UnpackRootfs(context.Background(), engineExt, rootfsPath, manifest, &unpackOptions); err != nil {
		return errors.Wrap(err, "create rootfs")
	}

	return nil
}

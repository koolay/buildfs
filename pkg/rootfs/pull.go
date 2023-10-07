package rootfs

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/image/v5/copy"
	dockerv5 "github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/go-logr/logr"
)

type ImagePuller struct {
	logger *logr.Logger
}

type PullOptions struct {
	Arch      string
	OS        string
	SrcImage  string
	DestImage string
}

func NewImagePuller(logger *logr.Logger) *ImagePuller {
	return &ImagePuller{logger: logger}
}

func (r *ImagePuller) Pull(
	ctx context.Context,
	options PullOptions,
	reporter io.Writer,
) error {
	imageTransport := dockerv5.Transport
	fmt.Println("imageTransport", fmt.Sprintf("//%s", options.SrcImage))
	srcRef, err := imageTransport.ParseReference(fmt.Sprintf("//%s", options.SrcImage))
	if err != nil {
		return fmt.Errorf("Error parsing source image reference: %w", err)
	}

	destRef, err := alltransports.ParseImageName(options.DestImage)
	if err != nil {
		return fmt.Errorf("invalid destination name %s: %w", options.DestImage, err)
	}

	imageListSelection := copy.CopySystemImage
	policy, err := getPolicyContext()
	if err != nil {
		return err
	}

	r.logger.Info("start pull image", "options", options)

	_, err = copy.Image(ctx, policy, destRef, srcRef, &copy.Options{
		RemoveSignatures:                 false,
		Signers:                          nil,
		SignBy:                           "",
		SignPassphrase:                   "",
		SignBySigstorePrivateKeyFile:     "",
		SignSigstorePrivateKeyPassphrase: []byte(""),
		SignIdentity:                     nil,
		ReportWriter:                     reporter,
		DestinationCtx:                   nil,
		ForceManifestMIMEType:            "",
		ImageListSelection:               imageListSelection,
		PreserveDigests:                  false,
		OciDecryptConfig:                 nil,
		OciEncryptLayers:                 nil,
		OciEncryptConfig:                 nil,
		SourceCtx: &types.SystemContext{
			ArchitectureChoice: options.Arch,
			OSChoice:           options.OS,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func getPolicyContext() (*signature.PolicyContext, error) {
	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	}
	return signature.NewPolicyContext(policy)
}

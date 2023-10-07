package rootfs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/koolay/buildfs/pkg/logging"
)

func TestImagePuller_pull(t *testing.T) {
	tag := "latest"
	srcImage := "docker://quay.io/jitesoft/alpine:" + tag
	destPath := "/tmp/buildfs/images/nginx"
	os.MkdirAll(destPath, 0755)
	defer os.RemoveAll(destPath)

	destImagePath := fmt.Sprintf("oci:%s:%s", destPath, tag)

	logger := logging.NewTestLog()
	puller := &ImagePuller{
		logger: &logger,
	}
	err := puller.Pull(context.Background(), srcImage, destImagePath, os.Stderr)
	assert.Nil(t, err)
	data, err := ioutil.ReadFile(filepath.Join(destPath, "index.json"))
	assert.Nil(t, err)
	assert.True(t, len(data) > 0)
}

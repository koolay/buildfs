package rootfs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/koolay/buildfs/pkg/logging"
)

func TestBuilder_CreateDiskImage(t *testing.T) {
	logger := logging.NewTestLog()
	puller := NewBuilder(&logger)
	ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
	defer cancel()

	workspaceDir := "/tmp/skopeo"
	os.Mkdir(workspaceDir, 0755)
	defer os.RemoveAll(workspaceDir)

	containerImage := "quay.io/jitesoft/alpine:latest"
	creds := PullCredentials{}
	got, err := puller.CreateDiskImage(ctx, workspaceDir, containerImage, creds)
	assert.Nil(t, err)
	if err != nil {
		panic(err)
	}
	fmt.Println("rootfs path", got)
	assert.True(t, len(got) > 0)
}

# Build rootfs from docker image

## How to run
```bash
go run main.go --image alpine:3.17 --workspace /tmp/buildfs

```

## Install 


```bash
$ go install github.com/koolay/buildfs@latest
```

OR

```bash

$ go install -tags "remote exclude_graphdriver_btrfs btrfs_noversion exclude_graphdriver_devicemapper containers_image_openpgp" \
    github.com/koolay/buildfs@latest
```



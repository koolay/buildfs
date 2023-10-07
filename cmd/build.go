/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/koolay/buildfs/pkg/logging"
	"github.com/koolay/buildfs/pkg/rootfs"
)

var rootfsFlags rootfs.Flags

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := logging.NewTestLog()
		puller := rootfs.NewBuilder(&logger)
		ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
		defer cancel()

		creds := rootfs.PullCredentials{}
		got, err := puller.CreateDiskImage(ctx, rootfsFlags.Workspace, rootfsFlags.ImageSrc, creds)
		if err != nil {
			panic(err)
		}

		fmt.Println("rootfs path", got)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().StringVar(&rootfsFlags.ImageSrc, "image", "", "image url, e.g. quay.io/jitesoft/alpine:latest")
	buildCmd.Flags().StringVar(&rootfsFlags.Workspace, "workspace", "", "workspace dir, e.g. /tmp/buildfs")
}

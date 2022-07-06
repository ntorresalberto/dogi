package cmd

import (
	// "context"
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	// "github.com/docker/docker/api/types"
	// "github.com/docker/docker/client"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: fmt.Sprintf("To debug %s!", appname),
	Long:  `This command provides an interface to test and see internal dogi information.`,
	Run: func(cmd *cobra.Command, args []string) {
		// ctx := context.Background()
		// cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		// check(err)

		// // images
		// imgs, err := cli.ImageList(ctx, types.ImageListOptions{})
		// check(err)
		// logger.Printf("docker images:")
		// for _, img := range imgs {
		// 	logger.Printf("%s: %d", img.RepoTags, img.Containers)
		// }

		// // containers
		// containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
		// check(err)
		// logger.Printf("docker containers:")
		// for _, container := range containers {
		// 	logger.Printf("%s %s\n", container.ID[:10], container.Image)
		// }

		logger.Println("debug command not implemented!")
		syscall.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}

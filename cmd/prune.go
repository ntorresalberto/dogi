package cmd

import (
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

const pruneExamples = `
  - Delete unused and/or dangling containers, images and volumes

    {{.appname}} prune
`

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "a docker space saver tool",
	Long: helpTemplate(`
It helps you clean up your PC from unused/dangling containers, images and volumes.
---------------------------------------------

Examples:

{{.pruneExamples}}
---------------------------------------------
`, map[string]string{"pruneExamples": pruneExamples}),
	Run: func(cmd *cobra.Command, args []string) {
		logger.Println("prune containers...")
		_, err := exec.Command("docker",
			"container", "prune", "-f").Output()
		check(err)

		logger.Println("prune images...")
		_, err = exec.Command("docker",
			"image", "prune", "-f").Output()
		check(err)

		logger.Println("prune volumes...")
		_, err = exec.Command("docker",
			"volume", "prune", "-f").Output()
		check(err)

		syscall.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}

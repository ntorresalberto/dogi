package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const pruneExamples = `
  - Delete unused and/or dangling containers, images and volumes

    {{.appname}} prune
`

func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1]
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "A docker space saver tool (deletes unused docker stuff)",
	Long: helpTemplate(`
It helps you clean up your PC from unused/dangling containers, images and volumes.
---------------------------------------------

Examples:

{{.pruneExamples}}
---------------------------------------------
`, map[string]string{"pruneExamples": pruneExamples}),
	Run: func(cmd *cobra.Command, args []string) {
		logger.Println("prune containers...")
		out, err := exec.Command("docker",
			"container", "prune", "-f").Output()
		fmt.Println(lastLine(out))
		check(err)

		logger.Println("prune images...")
		out, err = exec.Command("docker",
			"image", "prune", "-f").Output()
		fmt.Println(lastLine(out))
		check(err)

		logger.Println("prune volumes...")
		out, err = exec.Command("docker",
			"volume", "prune", "-f").Output()
		fmt.Println(lastLine(out))
		check(err)
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
}

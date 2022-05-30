package cmd

import (
	"fmt"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	// "github.com/ntorresalberto/dogi/assets"
)

// pruneCmd represents the prune command
var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "a docker prune wrapper",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("prune called")

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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pruneCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only prune when this command
	// is called directly, e.g.:
	// pruneCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

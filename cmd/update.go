package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
)

const updateExamples = `
  - Delete unused and/or dangling containers, images and volumes

    {{.appname}} update
`

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "a docker space saver tool",
	Long: helpTemplate(`
It updates {{.appname}}.
---------------------------------------------

Examples:

{{.updateExamples}}
---------------------------------------------
`, map[string]string{"updateExamples": updateExamples}),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Error: updated command not implemented yet.")
		syscall.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

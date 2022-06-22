package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func getCommitHash() string {
	out, err := exec.Command("git", "ls-remote", "https://"+githubUrl,
		"main").Output()
	if err != nil {
		fmt.Println(string(out))
		fmt.Println("Error: no internet?")
	}
	check(err)
	return strings.Fields(strings.TrimSpace(string(out)))[0]
}

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
		commitHash := getCommitHash()
		versionArg := fmt.Sprintf("-ldflags=\"-X github.com/ntorresalberto/dogi/cmd.Version=%s\"",
			commitHash)
		fmt.Printf("latest commit hash: %s\n", Gray(commitHash))

		updcmd := exec.Command("go",
			"install", versionArg, ".")
		updcmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
		)

		fmt.Printf("updating %s...", appname)
		out, err := updcmd.Output()
		if err != nil {
			fmt.Println(string(out))
			fmt.Println("update " + Red("FAILED"))
			fmt.Println("no internet?")
		}
		check(err)
		fmt.Println(Green("OK"))
		fmt.Println("check new version with: " +
			Gray(fmt.Sprintf("%s -v", appname)))
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
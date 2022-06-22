package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var cgoOn = false

func getCommitHash() string {
	out, err := exec.Command("git", "ls-remote", "https://"+githubUrl,
		"main").Output()
	if err != nil {
		fmt.Println(string(out))
		fmt.Println("Error: no internet?")
	}
	check(err)
	return strings.Fields(strings.TrimSpace(string(out)))[0][:8]
}

const updateExamples = `
  - Updates dogi to latest version

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
		versionArg := fmt.Sprintf("-X %s/cmd.Version=%s",
			githubUrl, commitHash)
		fmt.Printf("latest commit hash: %s\n", Gray(commitHash))

		updArgs := []string{"env"}
		if cgoOn {
			updArgs = append(updArgs, "CGO_ENABLED=0")

		}
		updArgs = append(updArgs, "go", "install", "-a",
			"-ldflags", versionArg, fmt.Sprintf("%s@latest", githubUrl))
		fmt.Println("command:", strings.Join(updArgs, " "))
		updcmd := exec.Command(updArgs[0], updArgs...)
		fmt.Printf("updating %s...", appname)
		out, err := updcmd.Output()
		if err != nil {
			fmt.Println(string(out))
			fmt.Println("exitCode:", updcmd.ProcessState.ExitCode())
			fmt.Println("err:", err)
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
	updateCmd.Flags().BoolVar(&cgoOn, "cgo", false, "don't use CGO_ENABLED=0 for go install command")
}

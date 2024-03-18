package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"syscall"
)

var (
	cgoOff        = false
	checkVersion  = false
	installCommit = ""
)

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

  - Updates dogi to specific version (branch or commit hash)

    {{.appname}} update --commit=aee8c7f
    {{.appname}} update --commit=main

  - Disable CGO in update:

    {{.appname}} update --no-cgo

  - Check if on the latest version

    {{.appname}} update --check
`

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: fmt.Sprintf("Update %s!", appname),
	Long: helpTemplate(`
It updates {{.appname}}.
---------------------------------------------

Examples:

{{.updateExamples}}
---------------------------------------------
`, map[string]string{"updateExamples": updateExamples}),
	Run: func(cmd *cobra.Command, args []string) {
		if installCommit == "" {
			installCommit = getCommitHash()
		}

		if checkVersion {
			fmt.Printf("installed version: %s\n", Gray(Version))
			fmt.Printf("   github version: %s\n", Gray(installCommit))
			if Version != installCommit {
				fmt.Printf("update available 🚩\n")
				fmt.Printf("use %s\n", Gray(fmt.Sprintf("%s update", appname)))
			} else {
				fmt.Printf("newest version installed ✅\n")
			}
			syscall.Exit(0)
		}

		// fmt.Println("len(installCommit)", len(installCommit))
		if len(installCommit) > 8 {
			installCommit = installCommit[:8]
		}
		fmt.Printf("install commit hash/branch: %s\n", Gray(installCommit))

		versionArg := fmt.Sprintf("-ldflags=-X %s/cmd.Version=%s",
			githubUrl, installCommit)

		updArgs := []string{"env"}
		if cgoOff {
			updArgs = append(updArgs, "CGO_ENABLED=0")

		}
		updArgs = append(updArgs, "go", "install", "-a",
			versionArg, fmt.Sprintf("%s@%s",
				githubUrl, installCommit))
		fmt.Println("command:", strings.Join(updArgs, " "))
		updcmd := exec.Command(updArgs[0], updArgs...)
		fmt.Printf("updating %s...", appname)
		out, err := updcmd.CombinedOutput()
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
	updateCmd.Flags().BoolVar(&cgoOff, "no-cgo", false, "don't use CGO (CGO_ENABLED=0) for go install command")
	updateCmd.Flags().BoolVar(&checkVersion, "check", false, "check if there's a newer version available")
	updateCmd.Flags().StringVar(&installCommit, "commit", "", "update to specific commit hash or branch. default: latest")
}

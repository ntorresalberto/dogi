package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const execExamples = `
  - Open a new terminal inside an existing container

    {{.appname}} exec

    {{.appname}} exec <container-name>
`

var (
	execCmd = &cobra.Command{
		Use:   "exec [flags] [container-name]",
		Short: "a docker exec wrapper",
		Long: helpTemplate(`
A docker exec wrapper.
It allows opening a new tty instance (like an interactive terminal) into an existing container.

---------------------------------------------

Examples:

{{.execExamples}}
---------------------------------------------
`, map[string]string{"execExamples": execExamples}),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(args)
			fmt.Println(len(args))

			nargs := len(args)
			contName := ""
			if nargs == 0 {
				out, err := exec.Command("docker", "ps").Output()
				check(err)

				options := strings.Split(
					strings.TrimSpace(string(out[:])), "\n")

				if len(options) == 0 {
					fmt.Printf("Error: no containers running?\n")
					syscall.Exit(1)
				}

				prompt := promptui.Select{
					Label: "Select Container",
					Items: options[1:],
				}

				_, result, err := prompt.Run()
				if err != nil {
					logger.Fatalf("select container failed")
				}

				logger.Printf("you choose %q\n", result)
				contName = strings.Fields(result)[0]
				logger.Printf("contName: %s", contName)

			} else if nargs == 1 {
				contName = args[0]
			} else {
				fmt.Printf("Error: exec command requires exactly 0 or 1 args (see example below)\n")
				fmt.Printf("       but %d were args provided: %s\n",
					nargs, strings.Join(args, " "))
				fmt.Println("       Please use the exec command like:")
				fmt.Printf("          - %s exec <container-name>\n", appname)
				fmt.Printf("          - %s exec\n", appname)
				fmt.Println("    (without arguments will ask you to choose between open containers)")
				syscall.Exit(1)
			}

			fmt.Printf("contName: %s\n", contName)

			if !noUserPtr {
				userObj, err := user.Current()
				check(err)

				logger.Println("username:", userObj.Username)
				dockerRunArgs = append(dockerRunArgs,
					fmt.Sprintf("--user=%s", userObj.Username))
			}
			if !workDirProvided() {
				// try to use the same workdir as when container was launched
				out, err := exec.Command("docker", "container",
					"inspect", "-f", "{{ .Config.WorkingDir }}", contName).Output()
				check(err)
				wd := strings.TrimSpace(string(out[:]))
				if wd != "" {
					workDirPtr = wd
				} else {
					// TODO: what should be default exec working dir? maybe ask?
					workDirPtr = "/"
				}
			}
			logger.Printf("workdir: %s\n", workDirPtr)
			dockerRunArgs = append(dockerRunArgs, fmt.Sprintf("--workdir=%s", workDirPtr))

			logger.Printf("docker args: %s\n", dockerRunArgs)

			entrypoint := []string{contName, "bash"}
			logger.Printf("entrypoint: %s\n", entrypoint)
			dockerArgs := merge([]string{dockerCmd, cmd.CalledAs()},
				dockerRunArgs,
				entrypoint)
			logger.Println("docker command: ", strings.Join(merge(dockerArgs), " "))

			// syscall exec is used to replace the current process
			check(syscall.Exec(dockerBinPath(), dockerArgs, os.Environ()))
		},
	}
)

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().BoolVar(&noUserPtr, "no-user", false, "don't use user inside container (run as root inside)")
	execCmd.Flags().StringVar(&workDirPtr, "workdir", "", "working directory inside the container")
}

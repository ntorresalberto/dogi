package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

const execExamples = `
  - Open a new terminal inside an existing container

    {{.appname}} exec

    {{.appname}} exec <container-name>

  - Run a command inside an existing container

    {{.appname}} exec -- make -C ~/myrepository/build

    {{.appname}} exec <container-name> -- make -C ~/myrepository/build
`

func userContainer(contName string) bool {
	out, err := exec.Command("docker", "container",
		"inspect", "-f", "{{ .Args  }}", contName).Output()
	check(err)
	return strings.Contains(string(out), appname)
}

func selectContainer() string {
	out, err := exec.Command("docker", "ps").Output()
	check(err)

	options := strings.Split(
		strings.TrimSpace(string(out[:])), "\n")
	if len(options) == 0 {
		fmt.Printf("Error: no containers running?\n")
		syscall.Exit(1)
	}

	result := ""
	prompt := &survey.Select{
		Message: "Select container:\n  " + options[0] + "\n",
		Options: options[1:],
	}
	if err := survey.AskOne(prompt, &result); err != nil {
		fmt.Println(err.Error())
		logger.Fatalf("select container failed")
	}

	contId := strings.Fields(result)[0]

	return contId
}

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
		PreRun: func(cmd *cobra.Command, args []string) {
			only1Arg(cmd, args, "container")
		},
		Run: func(cmd *cobra.Command, args []string) {
			// logger.Println("len(args):", len(args))
			// logger.Println("args:", args)
			// logger.Println("cmd.Flags().Args():", cmd.Flags().Args())
			contName := ""
			beforeArgs := beforeDashArgs(cmd, args)
			if len(beforeArgs) == 0 {
				contName = selectContainer()
				logger.Printf("contId: %s", contName)
			} else {
				contName = beforeArgs[0]
			}
			logger.Printf("contName: %s\n", contName)

			if !noUserPtr {
				if userContainer(contName) {
					userObj, err := user.Current()
					check(err)
					logger.Println("username:", userObj.Username)
					dockerRunArgs = append(dockerRunArgs,
						fmt.Sprintf("--user=%s", userObj.Username))
				} else {
					logger.Println("WARNING: container launched as root, won't use current user")
				}
			}
			if !workDirProvided() {
				// try to use the same workdir as when container was launched
				out, err := exec.Command("docker", "container",
					"inspect", "-f", "{{ .Config.WorkingDir }}", contName).Output()
				if err != nil {
					logger.Fatalf("container '%s' not available?", contName)
				}
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

			entrypoint := []string{contName}
			afterdargs := afterDashArgs(cmd, args)
			logger.Printf("afterDashArgs: %s\n", afterdargs)
			if len(afterdargs) > 0 {
				entrypoint = append(entrypoint, afterdargs...)
			} else {
				entrypoint = append(entrypoint, "bash")
			}
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

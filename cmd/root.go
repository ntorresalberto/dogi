package cmd

import (
	_ "embed"
	"log"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	// "fmt"
)

const (
	appname   = "dogi"
	githubUrl = "github.com/ntorresalberto/dogi"
	dockerCmd = "docker"
)

var (
	noUserPtr     bool
	noCacherPtr   bool
	homePtr       bool
	workDirPtr    string
	logger        = log.New(os.Stdout, appname+": ", log.Lshortfile)
	validCommands = [...]string{"run", "exec", "debug", "prune"}
	dockerRunArgs = []string{
		"--interactive",
		"--tty",
	}
	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "dogi",
		Short: "docker made easier!",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) {
		// 	fmt.Println("run rootCmd")
		// },
		SuggestionsMinimumDistance: 2,
	}
)

// find docker path for the exec command
func dockerBinPath() (dockerBinPath string) {
	dockerBinPath, err := exec.LookPath(dockerCmd)
	check(err)
	return
}

func merge(ss ...[]string) (s []string) {
	for kss := range ss {
		for k := range ss[kss] {
			s = append(s, ss[kss][k])
		}
	}
	return
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	var err error
	workDirPtr, err = os.Getwd()
	check(err)
	logger.Printf("current dir: %s\n", workDirPtr)

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

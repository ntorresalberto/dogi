package cmd

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"github.com/spf13/cobra"
)

const (
	appname   = "dogi"
	githubUrl = "github.com/ntorresalberto/dogi"
	dockerCmd = "docker"
)

func sessionPids() []int {
	out, err := exec.Command("ps", "-s").Output()
	check(err)
	lines := strings.Split(strings.TrimSpace(string(out[:])), "\n")

	pids := []int{}
	for _, line := range lines[1:] {
		pidField := strings.Fields(line)[1]
		pid64, err := strconv.ParseInt(pidField, 10, 32)
		if err != nil {
			fmt.Printf("Error: failed to strconv.ParseInt(%s, 10, 64)\n",
				pidField)
		}
		check(err)
		pids = append(pids, int(pid64))
	}
	sort.Ints(pids)
	return pids
}

func runInstance() string {
	ppid := os.Getppid()
	// fmt.Println("ppid:", ppid)
	pids := sessionPids()
	for k, pid := range pids {
		if ppid == pid && k > 0 {
			return Blue("(exec session)")
		}
		if pid > ppid {
			return Blue("(run session)")
		}
	}
	return ""
}

func Color(colorString string) func(...interface{}) string {
	sprint := func(args ...interface{}) string {
		return fmt.Sprintf(colorString,
			fmt.Sprint(args...))
	}
	return sprint
}

func insideContainer() bool {
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		return false
	}
	return true
}

var (
	Version       = "dev"
	privilegedPtr bool
	noUserPtr     bool
	noRMPtr       bool
	noNethostPtr  bool
	noCacherPtr   bool
	homePtr       bool
	workDirPtr    string
	contNamePtr   string
	logger        = log.New(os.Stdout, appname+": ", log.Lshortfile)
	dockerRunArgs = []string{
		"--interactive",
		"--tty",
	}
	Red     = Color("\033[1;31m%s\033[0m")
	Green   = Color("\033[1;32m%s\033[0m")
	Yellow  = Color("\033[1;33m%s\033[0m")
	Blue    = Color("\033[1;34m%s\033[0m")
	Gray    = Color("\033[1;37m%s\033[0m")
	rootCmd = &cobra.Command{
		Use:   "dogi",
		Short: "docker made easier!",
		Long: helpTemplate(`
{{.appname}} is a minimalist wrapper for docker run and docker exec to easily launch containers while sharing the working directory and use GUI applications.

{{.githubUrl}}

---------------------------------------------

Examples:

{{.execExamples}}
----------------
{{.runExamples}}
----------------
{{.pruneExamples}}
---------------------------------------------

`, map[string]string{"runExamples": runExamples,
			"execExamples": execExamples, "pruneExamples": pruneExamples}),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.CalledAs() != appname && insideContainer() {
				fmt.Printf("Error: %s cannot run inside a container\n", appname)
				syscall.Exit(1)
			}
		},
		// TODO: add multiple choice for help or check if inside container?
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				if insideContainer() {
					fmt.Println("You are " + Green("INSIDE") + " a container " + runInstance())
					if out, err := exec.Command("cat", "/proc/1/cpuset").Output(); err == nil {
						id := strings.TrimPrefix(strings.TrimSpace(string(out)),

							"/docker/")[:12]
						fmt.Println("container id: " + Green(id))
						fmt.Printf("open a new tty instance with: ")
						fmt.Println(Blue(fmt.Sprintf("%s exec %s", appname, id[:12])))
					}
				} else {
					fmt.Println("You are " + Yellow("OUTSIDE") + " a container (host machine)")
				}
				fmt.Println("to see examples and docs: " + Blue(fmt.Sprintf("%s help", appname)))
			}
		},
		SuggestionsMinimumDistance: 2,
		Version:                    Version,
	}
)

func panicKey(key string, mapWithoutKey map[string]string) {
	if _, ok := mapWithoutKey[key]; ok {
		panic(fmt.Errorf("%s should not exist in this dictionary\n", key))
	}
}

func helpTemplate(templ string, parameters map[string]string) string {

	const appnameKey = "appname"
	panicKey(appnameKey, parameters)
	parameters[appnameKey] = appname
	const githubKey = "githubUrl"
	panicKey(githubKey, parameters)
	parameters[githubKey] = githubUrl

	for key, val := range parameters {
		buf := &bytes.Buffer{}
		err := template.Must(template.New("").Option("missingkey=error").Parse(val)).Execute(buf,
			map[string]string{appnameKey: appname})
		check(err)
		parameters[key] = buf.String()
	}

	buf := &bytes.Buffer{}
	err := template.Must(template.New("").Option("missingkey=error").Parse(templ)).Execute(buf,
		parameters)
	check(err)
	return buf.String()
}

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

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func workDirProvided() bool {
	if workDirPtr == "" {
		// means flag was not provided
		var err error
		workDirPtr, err = os.Getwd()
		check(err)
		logger.Printf("current dir: %s\n", workDirPtr)
		return false
	}
	return true
}

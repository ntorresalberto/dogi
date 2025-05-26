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
	appname          = "dogi"
	githubUrl        = "github.com/ntorresalberto/dogi"
	dockerCmd        = "docker"
	cidFileContainer = "/" + appname + ".cid"
)

func announceEnteringContainer() {
	fmt.Println("going " + Green("inside") + " container, happy 🐳!")
}

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
			return Blue("(run session)") + string(" ⚡⚡")
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

func beforeDashArgs(cmd *cobra.Command, args []string) []string {
	if cmd.ArgsLenAtDash() != -1 {
		return args[:cmd.ArgsLenAtDash()]
	}
	return args
}

func afterDashArgs(cmd *cobra.Command, args []string) []string {
	if cmd.ArgsLenAtDash() == -1 {
		return []string{}
	}
	return args[cmd.ArgsLenAtDash():]
}

func only1Arg(cmd *cobra.Command, args []string, dockerType string) {
	maxArgs := 1
	beforeArgs := beforeDashArgs(cmd, args)
	if len(beforeArgs) > maxArgs {
		check(cmd.Help())
		fmt.Printf("\nError: %s %s was called with more than %d arguments (%s)\n",
			appname, cmd.CalledAs(),
			maxArgs, strings.Join(beforeArgs, " "))
		fmt.Printf("       but it can only be called with 0 or 1 argument (the docker %s)\n",
			dockerType)
		fmt.Println("       if you wanted to execute a specific command inside a container,")
		fmt.Println("       you need to use '--' like in the examples above")
		syscall.Exit(1)
	}
}

var (
	Version          = "dev"
	nvidiaRuntimePtr bool
	gpusAllPtr       bool
	privilegedPtr    bool
	noUserPtr        bool
	recentCtrPtr     bool
	noRMPtr          bool
	noUSBPtr         bool
	noNethostPtr     bool
	noCacherPtr      bool
	noSetupSudoPtr   bool
	pidIPCHostPtr    bool
	workDirPtr       string
	contNamePtr      string
	othPtr           string
	devAccPtr        string
	devRMWPtr        string
	tempDirPtr       string
	logger           = log.New(os.Stdout, appname+": ", log.Lmsgprefix)
	dockerRunArgs    = []string{
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

{{.runExamples}}
----------------
{{.execExamples}}
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
					if out, err := exec.Command("cat", cidFileContainer).Output(); err == nil {
						id := strings.TrimSpace(string(out))
						const maxStr = 12
						if len(id) > maxStr {
							id = id[:maxStr]
						}

						fmt.Println("container id: " + Green(id))
						fmt.Printf("open a new tty instance with: ")
						fmt.Println(Blue(fmt.Sprintf("%s exec %s", appname, id)))
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
		s = append(s, ss[kss]...)
	}
	return
}

func escapeSpaces(ss []string) (s []string) {
	ss2 := make([]string, len(ss))
	_ = copy(ss2, ss)
	for ks := range ss2 {
		spacespl := strings.Split(ss2[ks], " ")
		if len(spacespl) > 1 {
			eqspl := strings.Split(ss2[ks], "=")
			if len(eqspl) != 2 {
				panic(fmt.Sprintf("escapeSpaces: string should contain no more than a single '=':\n%v'", ss2[ks]))
			}
			ss2[ks] = fmt.Sprintf("%s='%s'", eqspl[0], eqspl[1])
		}

	}
	return ss2
}

func mergeEscapeSpaces(ss ...[]string) (s []string) {
	for kss := range ss {
		s = append(s, escapeSpaces(ss[kss])...)
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

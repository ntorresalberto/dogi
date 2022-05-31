package cmd

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/template"

	"github.com/spf13/cobra"
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
	dockerRunArgs = []string{
		"--interactive",
		"--tty",
	}
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
		// TODO: add multiple choice for help or check if inside container?
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) {
		// 	fmt.Println("run rootCmd")
		// },
		SuggestionsMinimumDistance: 2,
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

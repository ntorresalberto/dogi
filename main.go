package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"text/template"
)

// TODO: add note on why flags need to be placed after dogi
// the reason is related to the example:
// dogi run ubuntu bash -c "sudo apt install -y x11-apps && xeyes"
// in this case, "-c" will be parsed by the flag lib
// it's either after dogi or completely ignore non declared flag errors
const (
	appname   = "dogi"
	githubUrl = "github.com/ntorresalberto/dogi"
	usage     = `usage: {{.appname}}

{{.appname}} is a minimalist wrapper for docker run and docker exec to
easily launch containers while sharing the working directory and
use GUI applications.

Usage:

{{.appname}} [-flags] run docker-image [command]

{{.appname}} [-flags] exec [docker-image|docker-container]

{{.appname}} --version

{{.appname}} --help

NOTE: optional -flags must be placed right after {{.appname}}

Examples:

- Launch a container capable of GUI applications

{{.appname}} run ubuntu

- Launch a GUI command inside a container
xeyes is not installed in the ubuntu image by default.

{{.appname}} run ubuntu bash -c "sudo apt install -y x11-apps && xeyes"

- Launch an 3d accelerated GUI (opengl)

{{.appname}} run ubuntu bash -c "sudo apt install -y mesa-utils && glxgears"

Optional Flags:
`
)

var (
	logger        = log.New(os.Stdout, appname+".", log.Lshortfile)
	validCommands = [...]string{"run", "exec"}
	//go:embed createUser.sh.in
	createUserTemplate string
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func merge(ss ...[]string) (s []string) {
	for kss := range ss {
		for k := range ss[kss] {
			s = append(s, ss[kss][k])
		}
	}
	return
}
func main() {
	cwd, err := os.Getwd()
	check(err)
	noUserPtr := flag.Bool("no-user", false, "don't create user inside container (run as root inside)")
	versionPtr := flag.Bool("version", false, "show version")
	homePtr := flag.Bool("home", false, "mount your complete home directory")
	workDirPtr := flag.String("workdir", cwd, "working directory when launching the container, will be mounted inside")
	flag.Usage = func() {
		err := template.Must(template.New("").Option("missingkey=error").Parse(usage)).Execute(os.Stderr,
			map[string]string{"appname": appname})
		check(err)
		flag.PrintDefaults()
	}

	flag.Parse()

	if *versionPtr {
		fmt.Printf("%s dev version\n", appname)
		fmt.Println(githubUrl)
		return
	}

	logger.Println("tail:", flag.Args())
	logger.Println("*workDirPtr:", *workDirPtr)
	logger.Println("*homePtr:", *homePtr)
	logger.Println("*noUserPtr:", *noUserPtr)

	// validate arguments
	validCommand := false
	command := flag.Arg(0)
	for _, validCmd := range validCommands {
		if command == validCmd {
			validCommand = true
			continue
		}
	}
	if command == "" || !validCommand {
		flag.Usage()
		logger.Printf("\nError: please provide a valid command: %s \n",
			strings.Join(validCommands[:], ", "))
		syscall.Exit(1)
	}

	// find docker path for the exec command
	const dockerCmd string = "docker"
	dockerBinPath, err := exec.LookPath(dockerCmd)
	logger.Println("docker bin: ", dockerBinPath)
	check(err)

	dockerRunArgs := []string{
		"--interactive",
		"--tty",
		fmt.Sprintf("--workdir=%s", *workDirPtr),
	}

	// argument to be executed
	// (right after docker run or docker exec)
	entrypoint := []string{}

	switch command {
	// ********************************************************
	// ********************************************************
	case "exec":
		logger.Println("Error: sorry not implemented yet!")
		return

	// ********************************************************
	// ********************************************************
	case "run":

		logger.Println("flag.NArg():", flag.NArg())
		if flag.NArg() < 2 {
			flag.Usage()
			logger.Println("Error: docker image name not provided!")
			syscall.Exit(1)
		}
		imageName := flag.Arg(1)

		// find bash path
		bashCmdPath, err := exec.LookPath("bash")
		check(err)

		// create xauth magic cookie file
		xauthfile, err := os.CreateTemp("", fmt.Sprintf(".%s*.xauth", appname))
		check(err)
		logger.Println("temp file:", xauthfile.Name())
		// TODO: xauth file won't be removed because
		// process is replaced at Exec, is there a way?
		// defer os.Remove(xauthfile.Name())

		xauthCmdPath, err := exec.LookPath("xauth")
		check(err)

		const displayEnvVar string = "DISPLAY"
		displayEnv, ok := os.LookupEnv(displayEnvVar)
		if !ok {
			panic(fmt.Errorf("%s not set\n", displayEnvVar))
		}
		logger.Println("env DISPLAY:", displayEnv)

		xauthCmd := fmt.Sprintf("%s nlist %s | sed -e 's/^..../ffff/' | %s -f %s nmerge -",
			xauthCmdPath, displayEnv, xauthCmdPath, xauthfile.Name())
		logger.Println("xauth cmd:", xauthCmd)

		createXauthCmd := exec.Command(bashCmdPath, "-c", xauthCmd)
		check(createXauthCmd.Run())

		userObj, err := user.Current()
		check(err)
		logger.Println("username:", userObj.Username)
		logger.Println("    name:", userObj.Name)
		logger.Println("user uid:", userObj.Uid)
		logger.Println("user gid:", userObj.Gid)
		logger.Println("    home:", userObj.HomeDir)
		logger.Println("  groups:")
		relevantGroups := map[string]string{"video": "", userObj.Username: ""}
		groupIds, err := userObj.GroupIds()
		check(err)
		for k := range groupIds {
			gid := groupIds[k]
			group, err := user.LookupGroupId(gid)
			if err != nil {
				logger.Printf("    - gid %s not found\n", gid)
				continue
			}
			if _, ok := relevantGroups[group.Name]; ok {
				relevantGroups[group.Name] = group.Gid
			}
			logger.Printf("    - %s (%s)\n", group.Name, group.Gid)
		}
		// check gid where found
		createGroupsCmd := ""
		relevantGroupsIds := []string{}
		for key, val := range relevantGroups {
			if val == "" {
				panic(fmt.Errorf("user group '%s' not found", key))
			}
			temp := fmt.Sprintf("groupadd -g %s %s", val, key)
			relevantGroupsIds = append(relevantGroupsIds, val)
			if createGroupsCmd == "" {
				createGroupsCmd = temp
			} else {
				createGroupsCmd += " && " + temp
			}
		}

		mountStrs := []string{fmt.Sprintf("--volume=%s:%s", *workDirPtr, *workDirPtr)}
		if *homePtr {
			logger.Println("mounting home directory")
			mountStrs = append(mountStrs, fmt.Sprintf("--volume=%s:%s", userObj.HomeDir, userObj.HomeDir))
		}

		dockerRunArgs = append(dockerRunArgs, []string{
			"--rm",
			"--network=host",
			"--volume=/tmp/.X11-unix:/tmp/.X11-unix",
			fmt.Sprintf("--volume=%s:/.xauth", xauthfile.Name()),
			"--env=XAUTHORITY=/.xauth",
			"--env=QT_X11_NO_MITSHM=1",
			"--env=DISPLAY",
			"--env=TERM",
			"--volume=/etc/localtime:/etc/localtime:ro",
			"--device=/dev/dri",
		}...)
		dockerRunArgs = append(dockerRunArgs, mountStrs...)

		// figure out the command to execute (image default or provided)
		logger.Println("flag.Args():", flag.Args())
		logger.Println("flag.Args()[2:]:", flag.Args()[2:])
		execCommand := flag.Args()[2:]
		if len(execCommand) == 0 {
			// no command was provided, use image CMD
			out, err := exec.Command("docker",
				"inspect", "-f", "'{{join .Config.Cmd \",\"}}'", imageName).Output()
			check(err)
			execCommand = strings.Split(strings.Trim(strings.TrimSpace(string(out[:])),
				"'"), ",")
			logger.Println("imageCmd: [", strings.Join(execCommand, ", "), "]")
			if len(execCommand) == 0 {
				logger.Printf("%s has no CMD command? please report this as an issue!\n",
					imageName)
				execCommand = []string{"bash"}
			}
		}
		execCommandStr := strings.Join(execCommand, " ")
		logger.Println("execCommandStr:", execCommandStr)
		entrypoint = execCommand

		// create user script
		if !*noUserPtr {
			// TODO: createUser file won't be removed because
			// process is replaced at Exec, is there a way?
			createUserFile, err := os.CreateTemp("",
				fmt.Sprintf(".%s*.sh", appname))
			check(err)
			logger.Println("createUserFile:", createUserFile.Name())
			{
				err := template.Must(template.New("").Option("missingkey=error").Parse(createUserTemplate)).Execute(createUserFile,
					map[string]string{"username": userObj.Username,
						"homedir": userObj.HomeDir,
						"uid":     userObj.Uid,
						"ugid":    userObj.Gid,
						"gids":    strings.Join(relevantGroupsIds, ","),
						"Name":    userObj.Name,
					})
				check(err)
			}
			const createUserScriptPath = "/" + appname + "_create_user.sh"
			dockerRunArgs = append(dockerRunArgs,
				fmt.Sprintf("--volume=%s:%s", createUserFile.Name(),
					createUserScriptPath))
			entrypoint = merge([]string{"bash", createUserScriptPath}, execCommand)
		}

		dockerRunArgs = append(dockerRunArgs, imageName)
		// run command end
		// ********************************************************
		// ********************************************************
	}
	logger.Println("entrypoint:", entrypoint)

	dockerArgs := merge([]string{dockerCmd, command},
		dockerRunArgs,
		entrypoint)
	logger.Println("dockerArgs: [", strings.Join(dockerArgs, ", "), "]")
	for k := range dockerArgs {
		fmt.Println(k, dockerArgs[k])
	}

	logger.Println("docker cmd: ", strings.Join(merge(dockerArgs), " "))

	// syscall exec is used to replace the current process
	syscall.Exec(dockerBinPath, dockerArgs, os.Environ())
}

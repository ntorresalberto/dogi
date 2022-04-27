package main

import (
	_ "embed"
	"flag"
	"fmt"
	"github.com/google/shlex"
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
	appname = "dogi"
	usage   = `usage: {{.appname}}

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
	validCommands = [...]string{"run", "exec"}
	//go:embed createUser.sh.in
	createUserTemplate string
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func concat(ss ...[]string) (s string) {
	sa := []string{}
	for kss := range ss {
		for k := range ss[kss] {
			sa = append(sa, ss[kss][k])
		}
	}
	return strings.Join(sa, " ")
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
	fmt.Println("tail:", flag.Args())
	fmt.Println("*workDirPtr:", *workDirPtr)
	fmt.Println("*homePtr:", *homePtr)
	fmt.Println("*versionPtr:", *versionPtr)
	fmt.Println("*noUserPtr:", *noUserPtr)

	if *versionPtr {
		fmt.Printf("%s dev version\n", appname)
		return
	}

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
		fmt.Printf("\nError: please provide a valid command: %s \n",
			strings.Join(validCommands[:], ", "))
		syscall.Exit(1)
	}

	// find docker path for the exec command
	const dockerCmd string = "docker"
	dockerCmdPath, err := exec.LookPath(dockerCmd)
	fmt.Println("docker cmd: ", dockerCmdPath)
	check(err)

	dockerRunArgs := []string{
		"-it",
		fmt.Sprintf("--workdir=%s", *workDirPtr),
	}

	// argument to be executed
	// (right after docker run or docker exec)
	entrypoint := ""

	switch command {
	// ********************************************************
	// ********************************************************
	case "exec":
		fmt.Println("\nError: sorry not implemented yet!")
		return

	// ********************************************************
	// ********************************************************
	case "run":

		fmt.Println("flag.NArg():", flag.NArg())
		if flag.NArg() < 2 {
			flag.Usage()
			fmt.Println("\nError: docker image name not provided!")
			syscall.Exit(1)
		}
		imageName := flag.Arg(1)

		// find bash path
		bashCmdPath, err := exec.LookPath("bash")
		check(err)

		// create xauth magic cookie file
		xauthfile, err := os.CreateTemp("", fmt.Sprintf(".%s*.xauth", appname))
		check(err)
		fmt.Println("temp file:", xauthfile.Name())
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
		fmt.Println("env DISPLAY:", displayEnv)

		xauthCmd := fmt.Sprintf("%s nlist %s | sed -e 's/^..../ffff/' | %s -f %s nmerge -",
			xauthCmdPath, displayEnv, xauthCmdPath, xauthfile.Name())
		fmt.Println("xauth cmd:", xauthCmd)

		createXauthCmd := exec.Command(bashCmdPath, "-c", xauthCmd)
		check(createXauthCmd.Run())

		userObj, err := user.Current()
		check(err)
		fmt.Println("username:", userObj.Username)
		fmt.Println("    name:", userObj.Name)
		fmt.Println("user uid:", userObj.Uid)
		fmt.Println("user gid:", userObj.Gid)
		fmt.Println("    home:", userObj.HomeDir)
		fmt.Println("  groups:")
		relevantGroups := map[string]string{"video": "", userObj.Username: ""}
		groupIds, err := userObj.GroupIds()
		check(err)
		for k := range groupIds {
			fmt.Printf("    ")
			gid := groupIds[k]
			group, err := user.LookupGroupId(gid)
			if err != nil {
				fmt.Printf("- gid %s not found\n", gid)
				continue
			}
			if _, ok := relevantGroups[group.Name]; ok {
				relevantGroups[group.Name] = group.Gid
			}
			fmt.Printf("- %s (%s)\n", group.Name, group.Gid)
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

		mountStrs := []string{fmt.Sprintf("-v %s:%s", *workDirPtr, *workDirPtr)}
		if *homePtr {
			fmt.Println("mounting home directory")
			mountStrs = append(mountStrs, fmt.Sprintf("-v %s:%s", userObj.HomeDir, userObj.HomeDir))
		}

		dockerRunArgs = append(dockerRunArgs, []string{
			"--rm",
			"--network host",
			"-v /tmp/.X11-unix:/tmp/.X11-unix",
			fmt.Sprintf("-v %s:/.xauth", xauthfile.Name()),
			"-e XAUTHORITY=/.xauth",
			"-e DISPLAY",
			"-e TERM",
			"-e QT_X11_NO_MITSHM=1",
			"-v /etc/localtime:/etc/localtime:ro",
			"--device /dev/dri",
		}...)
		dockerRunArgs = append(dockerRunArgs, mountStrs...)

		// figure out the command to execute (image default or provided)
		fmt.Println("flag.Args():", flag.Args())
		fmt.Println("flag.Args()[2:]:", flag.Args()[2:])
		execCommand := flag.Args()[2:]
		// protect quoted arguments
		for k, val := range execCommand {
			execCommand[k] = "'" + val + "'"
		}
		execCommandStr := " "
		if len(execCommand) == 0 {
			// no command was provided, use image CMD
			out, err := exec.Command("docker",
				"inspect", "-f", "'{{.Config.Cmd}}'", imageName).Output()
			check(err)
			imageCmd := strings.Trim(strings.TrimSpace(string(out[:])), "'[]")
			fmt.Println("imageCmd:", imageCmd)
			if imageCmd != "" {
				execCommandStr += imageCmd
			} else {
				fmt.Printf("%s has no CMD command? please report this as an issue!\n",
					imageName)
				execCommandStr += "bash"
			}
		} else {
			execCommandStr += strings.Join(execCommand, " ")
		}
		fmt.Println("execCommandStr:", execCommandStr)
		entrypoint = execCommandStr

		// create user script
		if !*noUserPtr {
			// TODO: createUser file won't be removed because
			// process is replaced at Exec, is there a way?
			createUserFile, err := os.CreateTemp("",
				fmt.Sprintf(".%s*.sh", appname))
			check(err)
			fmt.Println("createUserFile:", createUserFile.Name())
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
				fmt.Sprintf("-v %s:%s", createUserFile.Name(),
					createUserScriptPath))
			entrypoint = fmt.Sprintf("bash -c \"bash %s %s\"",
				createUserScriptPath, execCommandStr)
		}

		dockerRunArgs = append(dockerRunArgs, imageName)
		// run command end
		// ********************************************************
		// ********************************************************
	}
	fmt.Println("entrypoint:", entrypoint)

	dockerArgsStr := concat([]string{dockerCmd, command},
		dockerRunArgs,
		[]string{entrypoint})
	fmt.Println("docker cmd: ", dockerArgsStr)
	dockerArgs, err := shlex.Split(dockerArgsStr + " " + entrypoint)
	check(err)

	// syscall exec is used to replace the current process
	syscall.Exec(dockerCmdPath, dockerArgs, os.Environ())
}

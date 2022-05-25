package main

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/manifoldco/promptui"
	flag "github.com/spf13/pflag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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

{{.appname}} is a minimalist wrapper for docker run and docker exec to easily launch containers while sharing the
working directory and use GUI applications.

---------------------------------------------

Usage:

  {{.appname}} [-flags] run docker-image [command]

  {{.appname}} [-flags] exec [docker-image|docker-container]

  {{.appname}} --version

  {{.appname}} --help

NOTE: optional -flags must be placed right after {{.appname}}


Optional Flags:

{{.flags}}

---------------------------------------------

Examples:


- Launch a container capable of GUI applications

{{.appname}} run ubuntu

----------------

- Launch a GUI command inside a container
xeyes is not installed in the ubuntu image by default.

{{.appname}} run ubuntu bash -c "sudo apt install -y x11-apps && xeyes"

----------------

- Launch an 3d accelerated GUI (opengl)

{{.appname}} run ubuntu bash -c "sudo apt install -y mesa-utils && glxgears"


`
)

var (
	logger        = log.New(os.Stdout, appname+": ", log.Lshortfile)
	validCommands = [...]string{"run", "exec", "debug", "prune"}
	//go:embed createUser.sh.in
	createUserTemplate string
	//go:embed apt-cacher-ng/Dockerfile
	aptCacheDockerfile string
)

func imgLocal(name string) bool {
	_, err := exec.Command("bash", "-c",
		fmt.Sprintf("docker images | grep %s", name)).Output()
	if err != nil {
		return false
	}
	return true
}

func setAptCacher() string {

	baseName := "apt-cacher"
	imgName := fmt.Sprintf("%s/%s", appname, baseName)

	if imgLocal(imgName) {
		logger.Printf("image %s found",
			imgName)
	} else {
		logger.Printf("%s NOT found, building...", imgName)
		dir, err := ioutil.TempDir("", "dogi_apt-cache")
		check(err)
		defer os.RemoveAll(dir) // clean up

		tmpfn := filepath.Join(dir, "Dockerfile")
		check(ioutil.WriteFile(tmpfn, []byte(aptCacheDockerfile), 0666))
		logger.Printf("temp dir: %s\n", dir)
		logger.Printf("temp Dockerfile: %s\n", tmpfn)

		cmd := exec.Command("docker",
			"build", "-t", imgName, ".")
		cmd.Dir = dir
		out, err := cmd.Output()
		logger.Printf(string(out))
		check(err)
	}

	// launch apt-cacher container
	contName := fmt.Sprintf("%s_%s_cont", appname, baseName)

	// find out apt-cacher ip
	out, err := exec.Command("docker",
		"inspect", "-f", "{{ .NetworkSettings.IPAddress }}'", contName).Output()
	if err != nil {
		logger.Printf("container %s not found, launching...", contName)
		out, err = exec.Command("docker",
			"run", "-d", "--restart=always",
			fmt.Sprintf("--volume=%s_%s_vol:/var/cache/apt-cacher-ng",
				appname, baseName),
			fmt.Sprintf("--name=%s", contName),
			imgName,
		).Output()
		logger.Printf(string(out))
		check(err)

		out, err = exec.Command("docker",
			"inspect", "-f", "{{ .NetworkSettings.IPAddress }}'", contName).Output()
		check(err)
	}
	ip := strings.Trim(strings.TrimSpace(string(out[:])), "'")
	if ip == "" {
		panic(fmt.Errorf("%s found but not running?", contName))
	}
	logger.Printf("container %s found: %s", contName, ip)

	aptCacherConf := fmt.Sprintf("Acquire::http { Proxy \"http://%s:3142\"; };", ip)

	aptCacherFile, err := os.CreateTemp("", fmt.Sprintf(".%s_%s_*", appname, baseName))
	logger.Printf("apt-cacher file: %s", aptCacherFile.Name())
	check(ioutil.WriteFile(aptCacherFile.Name(), []byte(aptCacherConf), 0666))
	return aptCacherFile.Name()
}

func createGroupCommand(gid, groupName string) string {
	return fmt.Sprintf("groupadd --gid %s %s", gid, groupName)
}

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
	noCacherPtr := flag.Bool("no-cacher", false, "don't launch apt-cacher container")
	versionPtr := flag.Bool("version", false, "show version")
	homePtr := flag.Bool("home", false, "mount your complete home directory")
	workDirPtr := flag.String("workdir", cwd, "working directory when launching the container, will be mounted inside")
	flag.Usage = func() {
		err := template.Must(template.New("").Option("missingkey=error").Parse(usage)).Execute(os.Stderr,
			map[string]string{"appname": appname,
				"flags": flag.CommandLine.FlagUsages()})
		check(err)
	}

	flag.Parse()

	if *versionPtr {
		fmt.Printf("%s dev version\n", appname)
		fmt.Println(githubUrl)
		return
	}

	// logger.Println("tail:", flag.Args())
	// logger.Println("*workDirPtr:", *workDirPtr)
	// logger.Println("*homePtr:", *homePtr)
	// logger.Println("*noUserPtr:", *noUserPtr)

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
		prompt := promptui.Select{
			Label: "Please provide a valid command",
			Items: validCommands[:],
		}
		_, result, err := prompt.Run()
		check(err)
		command = result
	}

	// find docker path for the exec command
	const dockerCmd string = "docker"
	dockerBinPath, err := exec.LookPath(dockerCmd)
	logger.Println("docker bin: ", dockerBinPath)
	check(err)

	dockerRunArgs := []string{
		"--interactive",
		"--tty",
	}

	userObj, err := user.Current()
	check(err)
	logger.Println("username:", userObj.Username)
	logger.Println("    name:", userObj.Name)
	logger.Println("user uid:", userObj.Uid)
	logger.Println("user gid:", userObj.Gid)
	logger.Println("    home:", userObj.HomeDir)

	// argument to be executed
	// (right after docker run or docker exec)
	entrypoint := []string{}

	switch command {
	// ********************************************************
	// ********************************************************
	case "prune":
		logger.Println("prune containers...")
		_, err := exec.Command("docker",
			"container", "prune", "-f").Output()
		check(err)

		logger.Println("prune images...")
		_, err = exec.Command("docker",
			"image", "prune", "-f").Output()
		check(err)

		logger.Println("prune volumes...")
		_, err = exec.Command("docker",
			"volume", "prune", "-f").Output()
		check(err)

		return

	// ********************************************************
	// ********************************************************
	case "debug":
		ctx := context.Background()
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		check(err)

		// images
		imgs, err := cli.ImageList(ctx, types.ImageListOptions{})
		check(err)
		logger.Printf("docker images:")
		for _, img := range imgs {
			logger.Printf("%s: %d", img.RepoTags, img.Containers)
		}

		// containers
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
		check(err)
		logger.Printf("docker containers:")
		for _, container := range containers {
			logger.Printf("%s %s\n", container.ID[:10], container.Image)
		}

		setAptCacher()
		logger.Printf("Error: sorry, command '%s' not implemented yet!",
			command)
		return

	// ********************************************************
	// ********************************************************
	case "exec":
		contName := ""
		switch flag.NArg() {
		case 1:
			out, err := exec.Command("docker", "ps").Output()
			check(err)

			options := strings.Split(
				strings.TrimSpace(string(out[:])), "\n")

			prompt := promptui.Select{
				Label: "Select Container",
				Items: options[1:],
			}

			_, result, err := prompt.Run()
			check(err)

			logger.Printf("you choose %q\n", result)
			contName = strings.Split(result, " ")[0]
			logger.Printf("contName: %s", contName)
		case 2:
			contName = flag.Arg(1)
			logger.Printf("contName: %s", contName)
		default:
			flag.Usage()
			fmt.Printf("Error: exec command requires exactly 0 or 1 args (see example below)\n")
			fmt.Printf("       but %d were args provided: %s\n",
				flag.NArg()-1, strings.Join(flag.Args()[1:], " "))
			fmt.Println("       Please use the exec command like:")
			fmt.Printf("        1 - %s exec <container-name>\n", appname)
			fmt.Printf("        2 - %s exec <image-name>\n", appname)
			fmt.Printf("        3 - %s exec\n", appname)
			fmt.Println("    (without arguments will ask you to choose between open containers)")
			syscall.Exit(1)
		}

		if !*noUserPtr {
			dockerRunArgs = append(dockerRunArgs,
				fmt.Sprintf("--user=%s", userObj.Username))
		}

		// TODO: add workdir through the flag argument, dealing with non-existant?
		entrypoint = []string{contName, "bash"}
		logger.Println("entrypoint: ", entrypoint)
		dockerArgs := merge([]string{dockerCmd, command},
			dockerRunArgs,
			entrypoint)
		logger.Println("docker command: ", strings.Join(merge(dockerArgs), " "))

		// syscall exec is used to replace the current process
		syscall.Exec(dockerBinPath, dockerArgs, os.Environ())

	// ********************************************************
	// ********************************************************
	case "run":
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
		logger.Println("temp xauth file:", xauthfile.Name())
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

		xauthCmd := fmt.Sprintf("%s nlist %s | sed -e 's/^..../ffff/' | %s -f %s nmerge -",
			xauthCmdPath, displayEnv, xauthCmdPath, xauthfile.Name())
		// logger.Println("xauth cmd:", xauthCmd)

		createXauthCmd := exec.Command(bashCmdPath, "-c", xauthCmd)
		check(createXauthCmd.Run())

		createGroupsCmd := createGroupCommand(userObj.Gid, userObj.Username)
		// TODO: apparently you can use --group-add video from docker run?
		// http://wiki.ros.org/docker/Tutorials/Hardware%20Acceleration#ATI.2FAMD
		toAddGroups := map[string]string{"video": ""}
		groupIds, err := userObj.GroupIds()
		check(err)
		toAddGids := []string{}
		// logger.Println("  groups:")
		for k := range groupIds {
			gid := groupIds[k]
			group, err := user.LookupGroupId(gid)
			if err != nil {
				logger.Printf("    - gid %s not found\n", gid)
				panic(err)
			}
			// logger.Printf("    - %s (%s)\n", group.Name, group.Gid)
			if _, ok := toAddGroups[group.Name]; ok {
				toAddGroups[group.Name] = group.Gid
				toAddGids = append(toAddGids, group.Gid)
				createGroupsCmd += " && " + createGroupCommand(group.Gid, group.Name)
			}
		}

		mountStrs := []string{fmt.Sprintf("--volume=%s:%s", *workDirPtr, *workDirPtr)}
		if *homePtr {
			logger.Println("mounting home directory")
			mountStrs = append(mountStrs, fmt.Sprintf("--volume=%s:%s", userObj.HomeDir, userObj.HomeDir))
		}

		dockerRunArgs = append(dockerRunArgs, []string{
			fmt.Sprintf("--workdir=%s", *workDirPtr),
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

		if !*noCacherPtr {
			logger.Println("using apt-cacher, disable with --no-cacher")
			dockerRunArgs = append(dockerRunArgs,
				fmt.Sprintf("--volume=%s:/etc/apt/apt.conf.d/01proxy", setAptCacher()))
		}

		// figure out the command to execute (image default or provided)
		// logger.Println("flag.Args():", flag.Args())
		// logger.Println("flag.Args()[2:]:", flag.Args()[2:])
		execCommand := flag.Args()[2:]
		if len(execCommand) == 0 {
			// no command was provided, use image CMD
			out, err := exec.Command("docker",
				"inspect", "-f", "'{{join .Config.Cmd \",\"}}'", imageName).Output()
			if err != nil {
				logger.Fatalf("docker inspect %s failed, image doesn't exist?", imageName)
			}

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
			logger.Println("create user script:", createUserFile.Name())
			{
				err := template.Must(template.New("").Option("missingkey=error").Parse(createUserTemplate)).Execute(createUserFile,
					map[string]string{"username": userObj.Username,
						"homedir": userObj.HomeDir,
						"uid":     userObj.Uid,
						"ugid":    userObj.Gid,
						"gids":    strings.Join(toAddGids, ","),
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
	logger.Println("docker command: ", strings.Join(merge(dockerArgs), " "))

	// syscall exec is used to replace the current process
	syscall.Exec(dockerBinPath, dockerArgs, os.Environ())
}

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/manifoldco/promptui"
	"github.com/ntorresalberto/dogi/assets"
	"github.com/spf13/cobra"
)

func timeZone() string {
	out, err := exec.Command("cat", "/etc/timezone").Output()
	check(err)
	return strings.TrimSpace(string(out[:]))
}

func imgLocal(name string) bool {
	_, err := exec.Command("bash", "-c",
		fmt.Sprintf("docker images | grep %s", name)).Output()
	return err == nil
}

type contState struct {
	exists, running bool
}

func contRunning(name string) contState {
	constate := contState{exists: true}
	out, err := exec.Command("docker", "container",
		"inspect", "-f", "{{ .State.Running }}", name).Output()
	if err != nil {
		constate.exists = false
	} else {
		constate.running = strings.TrimSpace(string(out[:])) == "true"
	}
	return constate
}

func setAptCacher() string {

	baseName := "apt-cacher"
	imgName := fmt.Sprintf("%s/%s", appname, baseName)

	{
		logger.Printf("build apt cacher image: %s\n", imgName)
		// build apt-cache-ng image
		dir, err := ioutil.TempDir("", "dogi_apt-cache")
		check(err)
		defer os.RemoveAll(dir) // clean up

		tmpfn := filepath.Join(dir, "Dockerfile")
		check(ioutil.WriteFile(tmpfn, []byte(assets.AptCacheDockerfile), 0666))
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

	contNeedsRestart := false
	constate := contRunning(contName)
	if constate.exists {
		// check container image is up to date
		out, err := exec.Command("docker", "image",
			"inspect", "-f", "{{ .Id }}", imgName).Output()
		check(err)
		imageId := strings.TrimSpace(string(out[:]))

		out, err = exec.Command("docker", "container",
			"inspect", "-f", "{{ .Image }}", contName).Output()
		contImageId := strings.TrimSpace(string(out[:]))

		if imageId != contImageId {
			logger.Printf("need to restart apt cache container")
			contNeedsRestart = true
		}
	}

	if contNeedsRestart {
		if constate.running {
			logger.Printf("container running, stopping...")
			_, err := exec.Command("docker", "container",
				"stop", contName).Output()
			check(err)
		}

		if constate.exists {
			logger.Printf("container exists, removing...")
			_, err := exec.Command("docker", "container",
				"rm", contName).Output()
			check(err)
		}
	}

	// find out apt-cacher ip
	out, err := exec.Command("docker", "container",
		"inspect", "-f", "{{ .NetworkSettings.IPAddress }}", contName).Output()
	if err != nil {
		logger.Printf("container %s not found, launching...", contName)
		out, err = exec.Command("docker",
			"run", "-d", "--restart=always",
			fmt.Sprintf("--volume=%s_%s_vol:/var/cache/apt-cacher-ng",
				appname, baseName),
			fmt.Sprintf("--name=%s", contName),
			imgName,
		).Output()
		logger.Printf("apt-cacher container started")
		check(err)

		out, err = exec.Command("docker", "container",
			"inspect", "-f", "{{ .NetworkSettings.IPAddress }}", contName).Output()
		check(err)
	}
	ip := strings.TrimSpace(string(out[:]))
	if ip == "" {
		panic(fmt.Errorf("%s found but not running?", contName))
	}
	logger.Printf("container %s found: %s", contName, ip)

	aptCacherConf := fmt.Sprintf("Acquire::http { Proxy \"http://%s:3142\"; };", ip)

	aptCacherFile, err := os.CreateTemp("", fmt.Sprintf(".%s_%s_*", appname, baseName))
	check(err)
	logger.Printf("apt-cacher file: %s", aptCacherFile.Name())
	check(ioutil.WriteFile(aptCacherFile.Name(), []byte(aptCacherConf), 0666))
	return aptCacherFile.Name()
}

func createGroupCommand(gid, groupName string) string {
	return fmt.Sprintf("groupadd --gid %s %s", gid, groupName)
}

const runExamples = `
  - Launch a container capable of GUI applications

    {{.appname}} run ubuntu

----------------

  - Launch a GUI command inside a container
  xeyes is not installed in the ubuntu image by default.

    {{.appname}} run ubuntu -- bash -c "sudo apt install -y x11-apps && xeyes"

----------------

  - Launch an 3D accelerated GUI (opengl)

 {{.appname}} run ubuntu -- bash -c "sudo apt install -y mesa-utils && glxgears"
`

var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "a docker run wrapper",
		Long: helpTemplate(`
{{.appname}} is a minimalist wrapper for docker run and docker exec to easily launch containers while sharing the
working directory and use GUI applications.

---------------------------------------------

Examples:

{{ .runExamples}}
---------------------------------------------
`, map[string]string{"runExamples": runExamples}),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			logger.Println("len(args):", len(args))
			logger.Println("args:", args)
			logger.Println("cmd.Flags().Args():", cmd.Flags().Args())
			var entrypoint []string
			imageName := ""
			if len(args) == 0 {
				out, err := exec.Command("docker", "images").Output()
				check(err)

				options := strings.Split(
					strings.TrimSpace(string(out[:])), "\n")

				prompt := promptui.Select{
					Label: "Select Image",
					Items: options[1:],
				}

				_, result, err := prompt.Run()
				check(err)
				imageName = strings.Split(result, " ")[0]
			} else {
				imageName = args[0]
			}

			logger.Printf("imageName: %s\n", imageName)

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

			userObj, err := user.Current()
			check(err)
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

			workDirProvided() // initializes working directory
			logger.Printf("workdir: %s\n", workDirPtr)
			mountStrs := []string{fmt.Sprintf("--volume=%s:%s", workDirPtr, workDirPtr)}
			if homePtr {
				logger.Println("mounting home directory")
				mountStrs = append(mountStrs, fmt.Sprintf("--volume=%s:%s", userObj.HomeDir, userObj.HomeDir))
			}
			dockerRunArgs = append(dockerRunArgs, []string{
				fmt.Sprintf("--workdir=%s", workDirPtr),
				"--rm",
				"--network=host",
				"--volume=/tmp/.X11-unix:/tmp/.X11-unix",
				fmt.Sprintf("--volume=%s:/.xauth", xauthfile.Name()),
				"--env=XAUTHORITY=/.xauth",
				"--env=QT_X11_NO_MITSHM=1",
				"--env=DISPLAY",
				"--env=TERM",
				// TODO: actually this should be setup by tzdata package
				// maybe it's better not to touch inside or set env var TZ?
				// https://bugs.launchpad.net/ubuntu/+source/tzdata/+bug/1554806
				fmt.Sprintf("--env=TZ=%s", timeZone()),
				// "--volume=/etc/localtime:/etc/localtime:ro",
				// "--volume=/etc/timezone:/etc/timezone:ro",
				"--device=/dev/dri",
			}...)
			dockerRunArgs = append(dockerRunArgs, mountStrs...)

			if !noCacherPtr {
				logger.Println("using apt-cacher, disable with --no-cacher")
				dockerRunArgs = append(dockerRunArgs,
					fmt.Sprintf("--volume=%s:/etc/apt/apt.conf.d/01proxy", setAptCacher()))
			}

			// figure out the command to execute (image default or provided)
			logger.Println("cmd.ArgsLenAtDash():", cmd.ArgsLenAtDash())

			var execCommand []string
			if cmd.ArgsLenAtDash() == -1 {
				// -- not provided means
				// no command was provided, use image CMD
				out, err := exec.Command("docker",
					"inspect", "-f", "{{join .Config.Cmd \",\"}}", imageName).Output()
				if err != nil {
					// TODO: fix this
					logger.Printf("Error: docker inspect %s failed, image doesn't exist?", imageName)
					logger.Fatalf("as a workaround, you can try executing this first: \ndocker pull %s", imageName)
				}

				execCommand = strings.Split(strings.Trim(strings.TrimSpace(string(out[:])),
					"'"), ",")
				logger.Println("imageCmd: [", strings.Join(execCommand, ", "), "]")
				if len(execCommand) == 0 {
					logger.Printf("%s has no CMD command? please report this as an issue!\n",
						imageName)
					execCommand = []string{"bash"}
				}
			} else {
				execCommand = args[cmd.ArgsLenAtDash():]
			}
			execCommandStr := strings.Join(execCommand, ", ")
			logger.Println("execCommand list:", execCommandStr)
			entrypoint = execCommand

			// create user script
			if !noUserPtr {
				// TODO: createUser file won't be removed because
				// process is replaced at Exec, is there a way?
				createUserFile, err := os.CreateTemp("",
					fmt.Sprintf(".%s*.sh", appname))
				check(err)
				logger.Println("create user script:", createUserFile.Name())
				{
					err := template.Must(template.New("").Option("missingkey=error").Parse(assets.CreateUserTemplate)).Execute(createUserFile,
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
			logger.Println("entrypoint:", entrypoint)

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
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&noUserPtr, "no-user", false, "don't use user inside container (run as root inside)")
	runCmd.Flags().StringVar(&workDirPtr, "workdir", "", "working directory when launching the container, will be mounted inside")
	runCmd.Flags().BoolVar(&noCacherPtr, "no-cacher", false, "don't launch apt-cacher container")
	runCmd.Flags().BoolVar(&homePtr, "home", false, "mount your complete home directory")
}

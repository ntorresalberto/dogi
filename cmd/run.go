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

	"github.com/AlecAivazis/survey/v2"
	"github.com/ntorresalberto/dogi/assets"
	"github.com/spf13/cobra"
)

func timeZone() string {
	out, err := exec.Command("timedatectl", "show").Output()
	check(err)
	return strings.Split(strings.Split(strings.TrimSpace(string(out[:])),
		"\n")[0], "=")[1]
}

type contState struct {
	exists, running bool
}

func imagesStartingWith(toComplete string) []string {
	out, err := exec.Command("docker", "images").Output()
	check(err)

	imglines := strings.Split(
		strings.TrimSpace(string(out[:])), "\n")
	images := []string{}
	for _, imgline := range imglines {
		fields := strings.Fields(imgline)
		imgtag := fields[0] + ":" + fields[1]
		if strings.HasPrefix(imgtag, toComplete) {
			images = append(images, imgtag)
		}
	}
	return images
}

func selectImage() string {

	out, err := exec.Command("docker", "images").Output()
	check(err)

	options := strings.Split(
		strings.TrimSpace(string(out[:])), "\n")

	if len(options) == 0 {
		fmt.Printf("Error: no images locally available?\n")
		syscall.Exit(1)
	}

	result := ""
	prompt := &survey.Select{
		Message: "Select an image:\n  " + options[0] + "\n",
		Options: options[1:],
	}
	if err := survey.AskOne(prompt, &result); err != nil {
		fmt.Println(err.Error())
		logger.Fatalf("select image failed")
	}

	imageId := strings.Fields(result)[2]

	return imageId
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
		if err != nil {
			fmt.Println(string(out))
			panic(err)
		}
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
		check(err)
		contImageId := strings.TrimSpace(string(out[:]))

		if imageId != contImageId {
			logger.Printf("need to restart apt cache container")
			contNeedsRestart = true
		}

		if !constate.running {
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
		_, err = exec.Command("docker",
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
		Use:   "run [docker-image]",
		Short: "a docker run wrapper",
		Long: helpTemplate(`
{{.appname}} is a minimalist wrapper for docker run and docker exec to easily launch containers while sharing the working directory and use GUI applications.

---------------------------------------------

Examples:

{{ .runExamples}}
---------------------------------------------
`, map[string]string{"runExamples": runExamples}),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return imagesStartingWith(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			maxArgs := 1
			beforeDashArgs := args
			if cmd.ArgsLenAtDash() != -1 {
				beforeDashArgs = args[:cmd.ArgsLenAtDash()]
			}
			if len(beforeDashArgs) > maxArgs {
				check(cmd.Help())
				fmt.Printf("\nError: %s %s was called with more than %d arguments (%s)\n",
					appname, cmd.CalledAs(),
					maxArgs, strings.Join(beforeDashArgs, " "))
				fmt.Printf("       but it can only be called with 0 or 1 argument (the docker image)\n")
				fmt.Println("       if you wanted to execute a specific command inside a container,")
				fmt.Println("       you need to use '--' like in the examples above")
				syscall.Exit(1)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			// logger.Println("len(args):", len(args))
			// logger.Println("args:", args)
			// logger.Println("cmd.Flags().Args():", cmd.Flags().Args())
			var entrypoint []string
			imageName := ""
			if len(args) == 0 {
				imageName = selectImage()
				logger.Printf("imageId: %s", imageName)

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
				displayEnv = ":0"
				logger.Printf("WARNING: env %s not set, using %s=%s\n",
					displayEnvVar, displayEnvVar, displayEnv)
			} else {
				logger.Printf("env %s=%s\n", displayEnvVar, displayEnv)
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

			driCard1Device := fmt.Sprintf("/dev/dri/card1")
			if _, err := os.Stat(driCard1Device); !os.IsNotExist(err) {
				logger.Printf("%s found, nvidia card? (3D might not work)\n",
					driCard1Device)
			}

			dogiPath, err := os.Executable()
			check(err)
			logger.Printf("dogi path:%s", dogiPath)

			dockerRunArgs = append(dockerRunArgs, []string{
				fmt.Sprintf("--workdir=%s", workDirPtr),
				"--network=host",
				"--volume=/tmp/.X11-unix:/tmp/.X11-unix",
				fmt.Sprintf("--volume=%s:/.xauth", xauthfile.Name()),
				fmt.Sprintf("--volume=%s:/usr/local/bin/%s", dogiPath, appname),
				"--env=XAUTHORITY=/.xauth",
				"--env=QT_X11_NO_MITSHM=1",
				fmt.Sprintf("--env=DISPLAY=%s", displayEnv),
				"--env=TERM",
				"--device=/dev/dri",
				// TODO: actually this should be setup by tzdata package
				// maybe it's better not to touch inside or set env var TZ?
				// https://bugs.launchpad.net/ubuntu/+source/tzdata/+bug/1554806
				fmt.Sprintf("--env=TZ=%s", timeZone()),
				// "--volume=/etc/localtime:/etc/localtime:ro",
				// "--volume=/etc/timezone:/etc/timezone:ro",
			}...)
			dockerRunArgs = append(dockerRunArgs, mountStrs...)

			// NOTE: this --security-opt is needed to avoid errors like:
			// dbus[1570]: The last reference on a connection was dropped without closing the connection.
			// This is a bug in an application. See dbus_connection_unref() documentation for details.
			// Most likely, the application was supposed to call dbus_connection_close(), since this is a private connection.
			// D-Bus not built with -rdynamic so unable to print a backtrace
			dockerRunArgs = append(dockerRunArgs, "--security-opt=apparmor:unconfined")

			if !noRMPtr {
				dockerRunArgs = append(dockerRunArgs, "--rm")
			}

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

				execCommand = strings.Split(strings.TrimSpace(string(out[:])), ",")
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
				// mount .ccache
				ccacheDir := fmt.Sprintf("%s/.ccache", userObj.HomeDir)
				if _, err := os.Stat(ccacheDir); !os.IsNotExist(err) {
					dockerRunArgs = append(dockerRunArgs,
						fmt.Sprintf("--volume=%s:%s", ccacheDir, ccacheDir))
				}

				// mount .ssh as read-only just in case
				sshDir := fmt.Sprintf("%s/.ssh", userObj.HomeDir)
				if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
					dockerRunArgs = append(dockerRunArgs,
						fmt.Sprintf("--volume=%s:%s:ro", sshDir, sshDir))
				}

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
	runCmd.Flags().BoolVar(&noRMPtr, "rm", false, "don't launch with --rm (container will exist after exiting)")
	runCmd.Flags().BoolVar(&homePtr, "home", false, "mount your complete home directory")
}

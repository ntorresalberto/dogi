package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
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

var copyToContainerFiles = map[string]string{}

var userSingletonInstance *userSingletonType

type userSingletonType struct {
	*user.User
}

func userSingleton() *userSingletonType {
	if userSingletonInstance == nil {
		userObj, err := user.Current()
		check(err)
		userSingletonInstance = &userSingletonType{userObj}

	}

	return userSingletonInstance
}

type createGroupsCommand struct {
	toAddGnames []string
	cmd         string
	gnames      string
}

func (m *userSingletonType) createGroupsCmd() createGroupsCommand {
	if m == nil {
		m = userSingleton()
	}

	groupsCmd := createGroupsCommand{}
	groupsCmd.cmd = createGroupCommandStr(m.Gid, m.Username)
	// TODO: apparently you can use --group-add video from docker run?
	// http://wiki.ros.org/docker/Tutorials/Hardware%20Acceleration#ATI.2FAMD
	toAddGroups := map[string]string{"video": "", "realtime": ""}
	groupIds, err := m.GroupIds()
	check(err)

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
			groupsCmd.toAddGnames = append(groupsCmd.toAddGnames, group.Name)
			groupsCmd.cmd += createGroupCommandStr(group.Gid, group.Name)
		}
	}
	for key, val := range toAddGroups {
		if val == "" {
			logger.Printf("user doesn't belong to group %s, won't add it to container", key)
		}
	}

	groupsCmd.gnames = strings.Join(groupsCmd.toAddGnames, ",")

	return groupsCmd
}

func isSameDir(dir1, dir2 string) bool {
	file_1, err_1 := os.Stat(dir1)

	if err_1 != nil {

		panic(err_1)

	}

	file_2, err_2 := os.Stat(dir2)

	if err_2 != nil {

		panic(err_2)

	}
	return os.SameFile(file_1, file_2)
}

func addCopyToContainerFile(srcpath, dstpath string) {
	if _, ok := copyToContainerFiles[srcpath]; !ok {
		copyToContainerFiles[srcpath] = dstpath
	} else {
		logger.Fatalf("%s already exists in copyToContainerFiles, please report this to the %s devs",
			srcpath, appname)
	}
}

func copyToContainer(srcpath, dstpath, dstcont string) {
	logger.Printf("cp %s -> %s:%s\n", srcpath, dstcont[:8], dstpath)
	dst := fmt.Sprintf("%s:%s", dstcont, dstpath)
	// fmt.Printf("docker cp -aL %s %s\n", srcpath, dst)
	out, err := exec.Command("docker", "cp", "-aL", srcpath, dst).CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
	}
	check(err)
}

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

func cargoImage(name string) string {
	out, err := exec.Command("docker",
		"inspect", "-f", "{{ .Config.Env }}", name).Output()
	check(err)
	outstrs := strings.Split(strings.TrimFunc(strings.TrimSpace(string(out)),
		func(a rune) bool { return a == '[' || a == ']' }), " ")
	cargoHome := ""
	for _, varStr := range outstrs {
		varstrspl := strings.Split(varStr, "=")
		envVar := varstrspl[0]
		if envVar == "CARGO_HOME" {
			cargoHome = varstrspl[1]
			break
		}
	}
	return cargoHome
}

var aptSupportedDistros = []string{"Ubuntu", "Debian"}

func supportedDistros() []string {
	return append(aptSupportedDistros, "Fedora")
}

func imageExists(imageName string) bool {
	_, err := exec.Command("docker", "image", "inspect",
		imageName).Output()
	return err == nil
}

func imageDistro(imageName string) string {
	out, err := exec.Command("docker", "run", "--rm", "--tty",
		imageName, "cat", "/etc/os-release").Output()
	check(err)

	for _, val := range supportedDistros() {
		if strings.Contains(string(out), val) {
			return val
		}
	}
	return ""
}

func aptCacherSupported(distro string) bool {
	for _, val := range aptSupportedDistros {
		if val == distro {
			return true
		}
	}
	return false
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

func createGroupCommandStr(gid, groupName string) string {
	createGroupTempl := `
outside_gid="{{.outside_gid}}"
outside_gname="{{.outside_gname}}"

echo "  - {{.outside_gname}} (gid {{.outside_gid}})"

warnings=0
group_exists=0

echo "    . check gid {{.outside_gid}} is valid..."
inside_gid_bygid=$(getent group "{{.outside_gid}}" | cut -f3 -d: || true)
inside_gid_bygname=$(getent group "{{.outside_gname}}" | cut -f3 -d: || true)
# echo "      inside_gid_bygid:${inside_gid_bygid}"
# echo "      inside_gid_bygname:${inside_gid_bygname}"

echo -n "     - gid (by gid): "
herewarn=0
if [ "${inside_gid_bygid}" ]; then
  group_exists=1
  if [ "${inside_gid_bygid}" != "{{.outside_gid}}" ]; then
    echo "WARNING"
    echo "      -> gid (by gid): exists inside container exists and differs from outside:"
    echo "      -> inside_gid_bygid:${inside_gid_bygid}, outside container: {{.outside_gid}}"
    warnings=1
    herewarn=1
  fi
fi
if [ "${herewarn}" == "0" ]; then
    echo "OK"
fi

echo -n "     - gid (by gname): "
herewarn=0
if [ "${inside_gid_bygname}" ]; then
  group_exists=1
  if [ "${inside_gid_bygname}" != "{{.outside_gid}}" ]; then
    echo "WARNING"
    echo "      -> gid (by gname) inside container exists and differs from outside:"
    echo "      -> inside_gid_bygname:${inside_gid_bygname}, outside container: {{.outside_gid}}"
    warnings=1
    herewarn=1
  fi
fi
if [ "${herewarn}" == "0" ]; then
    echo "OK"
fi
# ---------------------------------------------------------------------------

echo "    . check group name {{.outside_gname}} is valid..."
inside_gname_bygid=$(getent group "{{.outside_gid}}" | cut -f1 -d: || true)
inside_gname_bygname=$(getent group "{{.outside_gname}}" | cut -f1 -d: || true)
# echo "      inside_gname_bygid:${inside_gname_bygid}"
# echo "      inside_gname_bygname:${inside_gname_bygname}"

echo -n "     - groupname (by gid): "
herewarn=0
if [ "${inside_gname_bygid}" ]; then
  group_exists=1
  if [ "${inside_gname_bygid}" != "{{.outside_gname}}" ]; then
    echo "WARNING"
    echo "      -> groupname (by gid) exists inside container exists and differs from outside:"
    echo "      -> inside_gname_bygid:${inside_gname_bygid}, outside container: {{.outside_gname}}"
    warnings=1
    herewarn=1
  fi
fi
if [ "${herewarn}" == "0" ]; then
    echo "OK"
fi

echo -n "     - groupname (by gname): "
herewarn=0
if [ "${inside_gname_bygname}" ]; then
  group_exists=1
  if [ "${inside_gname_bygname}" != "{{.outside_gname}}" ]; then
    echo "WARNING"
    echo "      -> groupname (by gname) exists inside container exists and differs from outside:"
    echo "      -> inside_gname_bygname:${inside_gname_bygname}, outside container: {{.outside_gname}}"
    warnings=1
    herewarn=1
  fi
fi
if [ "${herewarn}" == "0" ]; then
    echo "OK"
fi

# echo "  group_exists: ${group_exists}"
# echo "        warnings: ${warnings}"
if [ "${warnings}" == "0" ]; then
  if [ "${group_exists}" == "0" ]; then
    echo "     => gid {{.outside_gid}} not found inside container, create"
    groupadd -g "{{.outside_gid}}" "{{.outside_gname}}";
  else
    echo "     => gid {{.outside_gid}} ({{.outside_gname}}) exists inside container"
  fi
else
  echo "    ---------------------------------"
  echo "    Warning: there were some issues with group {{.outside_gname}} ({{.outside_gid}}),"
  echo "    check log above but very often this does not pose a problem"
  echo "    (if it does create an issue with the running log output above)."
  echo "    ---------------------------------"
  # exit 1
fi
`
	var out bytes.Buffer
	err := template.Must(template.New("").Option("missingkey=error").Parse(createGroupTempl)).Execute(&out,
		map[string]string{"outside_gid": gid,
			"outside_gname": groupName,
		})
	check(err)

	return out.String()
}

const runExamples = `
  - Launch a container capable of GUI applications as user

    {{.appname}} run ubuntu

  - Launch a container capable of GUI applications as root

    {{.appname}} run --no-user ubuntu

  - Launch a GUI command inside a container
    (xeyes is not installed in the ubuntu image by default)

    {{.appname}} run ubuntu -- bash -c "sudo apt install -y x11-apps && xeyes"
    {{.appname}} run fedora -- bash -c "sudo dnf install -y xeyes && xeyes"

  - Launch an 3D accelerated GUI (opengl)

 {{.appname}} run ubuntu -- bash -c "sudo apt install -y mesa-utils && glxgears"
`

var (
	runCmd = &cobra.Command{
		Use:   "run [docker-image]",
		Short: "A docker run wrapper (launch new containers)",
		Long: helpTemplate(`
{{.appname}} is a minimalist wrapper for docker run and docker exec to easily launch containers while sharing the working directory and use GUI applications.

---------------------------------------------

Examples:

{{ .runExamples}}
---------------------------------------------
`, map[string]string{"runExamples": runExamples}),
		// FParseErrWhitelist: cobra.FParseErrWhitelist{
		// 	UnknownFlags: true,
		// },
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return imagesStartingWith(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			// fmt.Println("args:", args)
			// fmt.Println("cmd.Args:", cmd.Args)
			only1Arg(cmd, args, "image")
		},
		Run: func(cmd *cobra.Command, args []string) {
			// logger.Println("len(args):", len(args))
			// logger.Println("args:", args)
			// logger.Println("cmd.Flags().Args():", cmd.Flags().Args())
			var entrypoint []string
			imageName := ""
			beforeArgs := beforeDashArgs(cmd, args)
			if len(beforeArgs) == 0 {
				imageName = selectImage()
				logger.Printf("imageId: %s", imageName)

			} else {
				imageName = beforeArgs[0]
			}

			logger.Printf("imageName: %s\n", imageName)

			// find bash path
			bashCmdPath, err := exec.LookPath("bash")
			check(err)

			// create xauth magic cookie file
			xauthfile, err := os.CreateTemp("", fmt.Sprintf(".%s*.xauth", appname))
			check(err)
			logger.Println("temp xauth file:", xauthfile.Name())
			addCopyToContainerFile(xauthfile.Name(), "/.xauth")

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

			workDirProvided() // initializes working directory
			logger.Printf("workdir: %s\n", workDirPtr)
			mountStrs := []string{fmt.Sprintf("--volume=%s:%s", workDirPtr, workDirPtr)}

			cidFile := fmt.Sprintf("%s/.%s%v.cid", os.TempDir(), appname, rand.Int63())
			mountStrs = append(mountStrs, fmt.Sprintf("--cidfile=%s", cidFile))
			mountStrs = append(mountStrs, fmt.Sprintf("--volume=%s:%s", cidFile, cidFileContainer))

			driCard1Device := "/dev/dri/card1"
			if _, err := os.Stat(driCard1Device); !os.IsNotExist(err) {
				logger.Printf("%s found, nvidia card? (3D might not work)\n",
					driCard1Device)
			}

			dogiPath, err := os.Executable()
			check(err)
			logger.Printf("dogi path:%s", dogiPath)

			dockerRunArgs = append(dockerRunArgs, []string{
				fmt.Sprintf("--workdir=%s", workDirPtr),
				"--volume=/tmp/.X11-unix:/tmp/.X11-unix",
				"--env=XAUTHORITY=/.xauth",
				// either none or both together
				// ref: https://stackoverflow.com/a/35040140
				// NOTE: QT_X11_NO_MITSHM – stops Qt form using the MIT-SHM X11 extension.
				// "--env=QT_X11_NO_MITSHM=1",
				// "--env=QT_GRAPHICSSYSTEM=native",
				fmt.Sprintf("--env=DISPLAY=%s", displayEnv),
				"--env=TERM",
				"--device=/dev/dri",
				// needed for realtime kernel
				// https://stackoverflow.com/questions/47416870/checking-for-linux-capabilities-to-set-thread-priority
				"--userns=host",
				"--cap-add=SYS_NICE",
				// TODO: actually this should be setup by tzdata package
				// maybe it's better not to touch inside or set env var TZ?
				// https://bugs.launchpad.net/ubuntu/+source/tzdata/+bug/1554806
				fmt.Sprintf("--env=TZ=%s", timeZone()),
				// "--volume=/etc/localtime:/etc/localtime:ro",
				// "--volume=/etc/timezone:/etc/timezone:ro",
			}...)
			dockerRunArgs = append(dockerRunArgs, mountStrs...)

			if gpusAllPtr {
				dockerRunArgs = append(dockerRunArgs, "--gpus=all")
			}
			if nvidiaRuntimePtr {
				dockerRunArgs = append(dockerRunArgs, "--runtime=nvidia")
			}

			// if --name flag was provided
			if contNamePtr != "" {
				flag := fmt.Sprintf("--name=%s", contNamePtr)
				logger.Printf("set container name: %s", contNamePtr)
				dockerRunArgs = append(dockerRunArgs, flag)
			}

			if !noNethostPtr {
				logger.Println("adding --network=host")
				dockerRunArgs = append(dockerRunArgs, "--network=host")
			}

			if privilegedPtr {
				logger.Println("adding --privileged")
				dockerRunArgs = append(dockerRunArgs, "--privileged")
			}

			// NOTE: this --security-opt is needed to avoid errors like:
			// dbus[1570]: The last reference on a connection was dropped without closing the connection.
			// This is a bug in an application. See dbus_connection_unref() documentation for details.
			// Most likely, the application was supposed to call dbus_connection_close(), since this is a private connection.
			// D-Bus not built with -rdynamic so unable to print a backtrace
			dockerRunArgs = append(dockerRunArgs, "--security-opt=apparmor:unconfined")

			if !noRMPtr {
				dockerRunArgs = append(dockerRunArgs, "--rm")
			}

			if !imageExists(imageName) {
				logger.Printf("Error: docker image or tag '%s' doesn't exist?", imageName)
				logger.Printf("try using 'docker pull %s' first", imageName)
				logger.Fatalf("check: docker image inspect %s", imageName)
			}

			distro := imageDistro(imageName) // empty if not supported

			if aptCacherSupported(distro) {
				if !noCacherPtr {
					logger.Println("using apt-cacher, disable it with --no-cacher")
					file := setAptCacher()
					addCopyToContainerFile(file, "/etc/apt/apt.conf.d/01proxy")
				} else {
					logger.Println("disabling apt-cacher (--no-cacher=ON)")
				}
			} else {
				logger.Println("image is not apt-based, disabling apt-cacher (--no-cacher=ON)")
			}

			// figure out the command to execute (image default or provided)
			// logger.Println("cmd.ArgsLenAtDash():", cmd.ArgsLenAtDash())

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
			userObj := userSingleton()

			// mount cache vol
			dockerRunArgs = append(dockerRunArgs,
				fmt.Sprintf("--volume=%s_cache_vol:%s/.cache",
					appname, userObj.HomeDir))

			cargoHomeContDir := cargoImage(imageName)
			if cargoHomeContDir != "" {
				logger.Printf("found CARGO_HOME:%s", cargoHomeContDir)
				logger.Println("run cargo cache volume")
				cargoCacheArg := fmt.Sprintf("--volume=%s_cargo-cache_vol:%s/registry",
					appname, cargoHomeContDir)
				logger.Printf("found CARGO_HOME:%s", cargoHomeContDir)
				dockerRunArgs = append(dockerRunArgs, cargoCacheArg)
			}
			if !noUserPtr && userObj.Uid == "0" {
				logger.Printf("⚡⚡ WARNING: super user detected, did you use sudo?\n")
				logger.Printf("sudo dogi can only run with --no-user\n")
			} else if !noUserPtr && userObj.Uid != "0" {
				if distro == "" {
					logger.Printf("WARNING: '%s' is not based on a supported distro?\n", imageName)
					logger.Printf("ERROR: dogi only supports images based on these distros for now:\n")
					for _, val := range supportedDistros() {
						logger.Printf("  - %s\n", val)
					}
					logger.Printf("though you can run it as root with: --no-user\n")
					logger.Fatalf("%s run --no-user %s\n", appname, imageName)
				} else {
					logger.Printf("supported distro image detected: %s\n", distro)
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
					groupsCmd := userSingleton().createGroupsCmd()
					err := template.Must(template.New("").Option("missingkey=error").Parse(assets.CreateUserTemplate)).Execute(createUserFile,
						map[string]string{"username": userObj.Username,
							"homedir":      userObj.HomeDir,
							"uid":          userObj.Uid,
							"ugid":         userObj.Gid,
							"gnames":       groupsCmd.gnames,
							"Name":         userObj.Name,
							"createGroups": groupsCmd.cmd,
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

			dockerArgs := merge([]string{dockerCmd, "create"},
				dockerRunArgs,
				entrypoint)

			logger.Println("docker command: ", strings.Join(merge(dockerArgs), " "))

			out, err := exec.Command(dockerArgs[0], dockerArgs[1:]...).CombinedOutput()
			if err != nil {
				fmt.Println(string(out))
				syscall.Exit(1)
			}
			contId := strings.TrimSpace(string(out))

			addCopyToContainerFile(dogiPath, fmt.Sprintf("/usr/bin/%s", appname))
			for key, val := range copyToContainerFiles {
				copyToContainer(key, val, contId)
			}

			logger.Println("attach to container")
			logger.Printf("docker start -ai %s\n", contId[:12])

			if isSameDir(workDirPtr, userSingleton().HomeDir) {
				fmt.Println(Red("WARNING: current directory is HOME") + " (read below) ⚡⚡")
				fmt.Println("mounting home directory implies the container will use YOUR ~/.bashrc")
				fmt.Println("the recommended usage is to launch dogi from your source directory")
			}
			announceEnteringContainer()

			// syscall exec is used to replace the current process
			check(syscall.Exec(dockerBinPath(),
				[]string{"docker", "start", "-ai", contId}, os.Environ()))

		},
	}
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&noUserPtr, "no-user", false, "don't use user inside container (run as root inside)")
	runCmd.Flags().BoolVar(&nvidiaRuntimePtr, "runtime-nvidia", false, "add --runtime=nvidia")
	runCmd.Flags().BoolVar(&gpusAllPtr, "gpus-all", false, "add --gpus=all")
	runCmd.Flags().StringVar(&contNamePtr, "name", "", "change the container name")
	runCmd.Flags().StringVar(&workDirPtr, "workdir", "", "working directory when launching the container, will be mounted inside")
	runCmd.Flags().BoolVar(&privilegedPtr, "privileged", false, "add --privileged to docker run command")
	runCmd.Flags().BoolVar(&noCacherPtr, "no-cacher", false, "don't launch apt-cacher container")
	runCmd.Flags().BoolVar(&noRMPtr, "no-rm", false, "don't launch with --rm (container will exist after exiting)")
	runCmd.Flags().BoolVar(&noNethostPtr, "no-nethost", false, "don't launch with --network=host")
}

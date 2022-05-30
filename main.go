package main

import (
	_ "embed"

	"github.com/ntorresalberto/dogi/cmd"
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

func main() {
	cmd.Execute()
}

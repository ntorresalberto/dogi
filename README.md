<p align="center">
<img src=https://user-images.githubusercontent.com/63748204/165713084-59b79373-7c7f-4309-86ce-6991230f8fbb.png width=350/>
</p>

# dogi

[![Go Reference](https://pkg.go.dev/badge/github.com/ntorresalberto/dogi.svg)](https://pkg.go.dev/github.com/ntorresalberto/dogi)
[![Go Report Card](https://goreportcard.com/badge/github.com/ntorresalberto/dogi)](https://goreportcard.com/report/github.com/ntorresalberto/dogi)
[![Build](https://github.com/ntorresalberto/dogi/actions/workflows/quality_build.yml/badge.svg)](https://github.com/ntorresalberto/dogi/actions)
[![Releases](https://github.com/ntorresalberto/dogi/actions/workflows/release.yml/badge.svg)](https://github.com/ntorresalberto/dogi/releases/tag/rolling)

**dogi** is a [simple and transparent](#design-principles) wrapper for `docker run` (and `docker exec`) to make common tasks easier.
It allows using rootless containers, running GUIs, quickly mounting your current directory and much more!

Even though **dogi** was originally inspired by [rocker](https://github.com/osrf/rocker) and solves a similar problem (or the same), it aims to do so with minimum user effort. Additionally, it provides the ability to interact with the `docker` client directly ([transparent](#design-principles)).

## !!! WARNING !!!

* Dogi's processes assume that Docker is able to build an image in the /tmp folder. This is typically verified if Docker is installed via the procedure described on the following pages: https://docs.docker.com/engine/install/ubuntu/ ; https://docs.docker.com/engine/install/linux-postinstall/ . This is typically not checked if Docker is installed via snap. 

* For now, Dogi only works for images based on these distros: Ubuntu, Debian, Fedora. Dogi also won't work for images that are too simplistic (typically, the “hello-world” Docker image).

* In order to use Docker (and thus, Dogi) with GPU support, you need to follow the installation prerequisites instructions (https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#prerequisites), and the subsequents configuration instructions (https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html#configuration). Otherwise, the following bug will appears : 

```
Bug : Error response from daemon: unknown or invalid runtime name: nvidia
```

## Quickstart

First, do : 

```
## To install this fork
export DOGI_INSTALL_GITHUB="XV25"
## To install the original repo
export DOGI_INSTALL_GITHUB="ntorresalberto"
```

Then :

```bash
# install binary
mkdir -pv ~/go/bin
wget -qO- https://github.com/$DOGI_INSTALL_GITHUB/dogi/releases/download/rolling/dogi-rolling-linux-amd64.tar.gz | tar xvz -C ~/go/bin

# add bash autocompletion
echo 'source <(dogi completion bash)' >> ~/.bashrc

# try it!
dogi run ubuntu

# update it easily
dogi update
```

Some [optional setup steps](#optional-setup-steps) might be required.

**NOTE:** You can also install from source: `CGO_ENABLED=0 go install -a github.com/ntorresalberto/dogi@latest`

---

- [Examples](#examples)
- [Overview](#overview)
  - [For whom?](#for-whom)
  - [Design principles](#design-principles)
  - [Limitations](#limitations)
- [For Developers](#for-developers)
  - [Compiling from source](#compiling-from-source)
- [Optional Setup Steps](#optional-setup-steps)

<hr style="border:3px solid blue">

### Examples

- Update dogi

```bash
    dogi update
    dogi update --commit=aee8c7f
```

- Launch a container capable of GUI applications

```bash
    dogi run ubuntu
    dogi run --no-user ubuntu # as root
    dogi run --home ubuntu # share your home directory inside container
```

- Open a new terminal inside an existing container

```bash
    dogi exec
    dogi exec --no-user # as root
    dogi exec <container-name>
```


- Launch a GUI command inside a container
(`xeyes` is not installed in the `ubuntu` image by default)

```bash
    dogi run ubuntu -- bash -c "sudo apt install -y x11-apps && xeyes"
    dogi run ubuntu --no-user -- bash -c "apt update && apt install -y x11-apps && xeyes" # as root

    dogi run fedora -- bash -c "sudo dnf install -y xeyes && xeyes"
    dogi run fedora --no-user -- bash -c "dnf install -y xeyes && xeyes" # as root
```

- Launch an 3D accelerated GUI (opengl)

```bash
    dogi run ubuntu -- bash -c "sudo apt install -y mesa-utils && glxgears"
    dogi run ubuntu --no-user -- bash -c "apt install -y mesa-utils && glxgears" # as root
```

- Add access to a webcam (ex : /dev/video0) (after having given access, for example, through ```  sudo usermod -aG video $USER```) : 

```bash
	dogi run ubuntu --device-access "/dev/video0"
```


- Delete unused and/or dangling containers, images and volumes

```bash
    dogi prune
```

<hr style="border:4px solid blue">

## Overview
### For whom?

You should find **dogi** useful if you:

- Run GUI applications inside docker containers
- Want to use containers as a development environment
- Run 3D accelerated applications inside docker containers (like opengl)
- Want to avoid using root inside containers instead of the default root
- Simply need to quickly mount your current directory to test something

### Design principles

- **transparent**: **dogi** forwards any unrecognized arguments to docker, in case you ever need to do anything not currently supported.
- **simple**: aims to cover the most common use cases with the least user intervention (you shouldn't need to pass any extra flags/options most of the time). If you don't agree with the defaults, [please say so](https://github.com/ntorresalberto/dogi/issues/new).
- **secure**: there are [many ways](http://wiki.ros.org/docker/Tutorials/GUI) to expose the xorg server to containers, **dogi** tries to do it in the most secure way. Additionally, it proposes an easy way to avoid the potentially dangerous practice of root containers. 
- **minimalist**: **dogi** thrives to have the least amount of dependencies and not do more than it needs.

> Many (open source) hackers are proud if they achieve large amounts of code, because they believe the more lines of code they've written, the more progress they have made. The more progress they have made, the more skilled they are. This is simply a delusion.

[from the suckless.org Manifest](https://suckless.org/philosophy/)

### Limitations

- Only supports ubuntu-based images (because of apt commands used)
- Only supports X11 environments for GUI applications (because of xorg socket communication)

<hr style="border:4px solid blue">

## For developers

### Installing from source

First, do : 

```
## To install this fork
export DOGI_INSTALL_GITHUB="XV25"
## To install the original repo
export DOGI_INSTALL_GITHUB="ntorresalberto"
```

Then :

```bash
git clone https://github.com/$DOGI_INSTALL_GITHUB/dogi.git
cd dogi
make install
dogi -v         # test it!
```

Once installed, add autocompletion with:
```bash
echo 'source <(dogi completion bash)' >> ~/.bashrc
```

### Optional setup steps

**installing go**

You need [golang>=1.16](https://go.dev/doc/install) installed (check version with `go version`).
On ubuntu, an updated version can easily be installed with **one of these** 2 ways:
```bash
sudo apt install golang         # ubuntu >= 22.04
sudo snap install go --classic
```

<hr style="border:1px solid blue">

**bash: dogi: command not found**

This error message means your `$PATH` doesn't include the go binaries path.
You can fix it [like this](https://stackoverflow.com/questions/42965673/cant-run-go-bin-in-terminal) or, if you only want to enable **dogi**, use:
```bash
echo "alias dogi=$(go env GOPATH)/bin/dogi" >> ~/.bashrc
source .bashrc
```

<hr style="border:1px solid blue">


**dogi: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.32' not found (required by dogi)**

This error is usually caused by a container running an older version of glibc than your host system (where you compiled `dogi`).
A possible cause of this is you didn't use `CGO_ENABLED=0` in the `go install`, as specified in #quickstart.


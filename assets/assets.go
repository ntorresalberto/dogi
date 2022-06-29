package assets

import _ "embed"

var (
	//go:embed createUser.sh.in
	CreateUserTemplate string
	//go:embed apt-cacher/Dockerfile
	AptCacheDockerfile string
)

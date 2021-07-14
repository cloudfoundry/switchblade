package switchblade

import "fmt"

type Platform struct {
	Deploy DeployProcess
	Delete DeleteProcess
}

type DeployProcess interface {
	WithBuildpacks(buildpacks ...string) DeployProcess
	WithEnv(env map[string]string) DeployProcess
	WithoutInternetAccess() DeployProcess

	Execute(name, path string) (Deployment, fmt.Stringer, error)
}

type DeleteProcess interface {
	Execute(name string) error
}

package switchblade

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
)

//go:generate faux --interface Executable --output fakes/executable.go
type Executable interface {
	Execute(pexec.Execution) error
}

func NewCloudFoundry(cli Executable, tempDir, homeDir string) Platform {
	return Platform{
		Deploy: CloudFoundryDeployProcess{cli: cli, tempDir: tempDir, homeDir: homeDir, internetAccess: true},
		Delete: CloudFoundryDeleteProcess{cli: cli, tempDir: tempDir},
	}
}

type CloudFoundryDeployProcess struct {
	cli     Executable
	tempDir string
	homeDir string

	buildpacks     []string
	env            map[string]string
	internetAccess bool
}

func (p CloudFoundryDeployProcess) WithBuildpacks(buildpacks ...string) DeployProcess {
	p.buildpacks = buildpacks
	return p
}

func (p CloudFoundryDeployProcess) WithEnv(env map[string]string) DeployProcess {
	p.env = env
	return p
}

func (p CloudFoundryDeployProcess) WithoutInternetAccess() DeployProcess {
	p.internetAccess = false
	return p
}

func (p CloudFoundryDeployProcess) Execute(name, source string) (Deployment, fmt.Stringer, error) {
	home := filepath.Join(p.tempDir, name)
	err := os.MkdirAll(home, os.ModePerm)
	if err != nil {
		return Deployment{}, nil, err
	}

	err = fs.Copy(p.homeDir, filepath.Join(home, ".cf"))
	if err != nil {
		return Deployment{}, nil, err
	}

	env := append(os.Environ(), fmt.Sprintf("CF_HOME=%s", home))
	log := bytes.NewBuffer(nil)
	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"create-org", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to create-org: %w\n\nOutput:\n%s", err, log)
	}

	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"create-space", name, "-o", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to create-space: %w\n\nOutput:\n%s", err, log)
	}

	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"target", "-o", name, "-s", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to target: %w\n\nOutput:\n%s", err, log)
	}

	configFile, err := os.Open(filepath.Join(home, ".cf", "config.json"))
	if err != nil {
		panic(err)
	}
	defer configFile.Close()

	var config struct {
		Target string
	}
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		return Deployment{}, nil, err
	}

	target, err := url.Parse(config.Target)
	if err != nil {
		return Deployment{}, nil, err
	}

	securityGroup := PublicSecurityGroup
	if !p.internetAccess {
		securityGroup = PrivateSecurityGroup
	}

	addrs, err := net.LookupHost(target.Host)
	if err != nil {
		return Deployment{}, nil, err
	}

	for _, addr := range addrs {
		if !strings.Contains(addr, ":") {
			securityGroup = append(securityGroup, SecurityGroupRule{
				Destination: addr,
				Protocol:    "all",
			})
		}
	}

	content, err := json.Marshal(securityGroup)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filepath.Join(home, "security-group.json"), content, 0600)
	if err != nil {
		return Deployment{}, nil, err
	}

	err = os.WriteFile(filepath.Join(home, "empty-security-group.json"), []byte("[]"), 0600)
	if err != nil {
		panic(err)
	}

	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"create-security-group", name, filepath.Join(home, "security-group.json")},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to create-security-group: %w\n\nOutput:\n%s", err, log)
	}

	// bind sec group
	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"bind-security-group", name, name, name, "--lifecycle", "staging"},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to bind-security-group: %w\n\nOutput:\n%s", err, log)
	}

	// update sec group
	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"update-security-group", "public_networks", filepath.Join(home, "empty-security-group.json")},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to update-security-group: %w\n\nOutput:\n%s", err, log)
	}

	args := []string{"push", name, "-p", source, "--no-start"}
	for _, buildpack := range p.buildpacks {
		args = append(args, "-b", buildpack)
	}

	err = p.cli.Execute(pexec.Execution{
		Args:   args,
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to push: %w\n\nOutput:\n%s", err, log)
	}

	var keys []string
	for key := range p.env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		err = p.cli.Execute(pexec.Execution{
			Args:   []string{"set-env", name, key, p.env[key]},
			Stdout: log,
			Stderr: log,
			Env:    env,
		})
		if err != nil {
			return Deployment{}, log, fmt.Errorf("failed to set-env: %w\n\nOutput:\n%s", err, log)
		}
	}

	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"start", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to start: %w\n\nOutput:\n%s", err, log)
	}

	buffer := bytes.NewBuffer(nil)
	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"app", name, "--guid"},
		Stdout: buffer,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to fetch guid: %w\n\nOutput:\n%s", err, buffer)
	}

	guid := strings.TrimSpace(buffer.String())
	buffer = bytes.NewBuffer(nil)
	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"curl", path.Join("/v2", "apps", guid, "routes")},
		Stdout: buffer,
		Env:    env,
	})
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to fetch routes: %w\n\nOutput:\n%s", err, buffer)
	}

	var routes struct {
		Resources []struct {
			Entity struct {
				DomainURL string `json:"domain_url"`
				Host      string `json:"host"`
				Path      string `json:"path"`
			} `json:"entity"`
		} `json:"resources"`
	}
	err = json.NewDecoder(buffer).Decode(&routes)
	if err != nil {
		return Deployment{}, log, fmt.Errorf("failed to parse routes: %w\n\nOutput:\n%s", err, buffer)
	}

	deployment := Deployment{Name: name}

	if len(routes.Resources) > 0 {
		route := routes.Resources[0].Entity
		buffer = bytes.NewBuffer(nil)
		err = p.cli.Execute(pexec.Execution{
			Args:   []string{"curl", route.DomainURL},
			Stdout: buffer,
			Env:    env,
		})
		if err != nil {
			return Deployment{}, log, fmt.Errorf("failed to fetch domain: %w\n\nOutput:\n%s", err, buffer)
		}

		var domain struct {
			Entity struct {
				Name string `json:"name"`
			} `json:"entity"`
		}
		err = json.NewDecoder(buffer).Decode(&domain)
		if err != nil {
			return Deployment{}, log, fmt.Errorf("failed to parse domain: %w\n\nOutput:\n%s", err, buffer)
		}

		deployment.URL = fmt.Sprintf("http://%s.%s%s", route.Host, domain.Entity.Name, route.Path)
	}

	return deployment, log, nil
}

type CloudFoundryDeleteProcess struct {
	cli     Executable
	tempDir string
}

func (p CloudFoundryDeleteProcess) Execute(name string) error {
	home := filepath.Join(p.tempDir, name)
	env := append(os.Environ(), fmt.Sprintf("CF_HOME=%s", home))
	log := bytes.NewBuffer(nil)
	err := p.cli.Execute(pexec.Execution{
		Args:   []string{"delete-org", name, "-f"},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to delete-org: %w\n\nOutput:\n%s", err, log)
	}

	err = p.cli.Execute(pexec.Execution{
		Args:   []string{"delete-security-group", name, "-f"},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to delete-security-group: %w\n\nOutput:\n%s", err, log)
	}

	err = os.RemoveAll(home)
	if err != nil {
		panic(err)
	}

	return nil
}

type SecurityGroupRule struct {
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	Ports       string `json:"ports,omitempty"`
}

var (
	PublicSecurityGroup = []SecurityGroupRule{
		{
			Destination: "0.0.0.0-9.255.255.255",
			Protocol:    "all",
		},
		{
			Destination: "11.0.0.0-169.253.255.255",
			Protocol:    "all",
		},
		{
			Destination: "169.255.0.0-172.15.255.255",
			Protocol:    "all",
		},
		{
			Destination: "172.32.0.0-192.167.255.255",
			Protocol:    "all",
		},
		{
			Destination: "192.169.0.0-255.255.255.255",
			Protocol:    "all",
		},
	}

	PrivateSecurityGroup = []SecurityGroupRule{
		{
			Protocol:    "tcp",
			Destination: "10.0.0.0-10.255.255.255",
			Ports:       "443",
		},
		{
			Protocol:    "tcp",
			Destination: "172.16.0.0-172.31.255.255",
			Ports:       "443",
		},
		{
			Protocol:    "tcp",
			Destination: "192.168.0.0-192.168.255.255",
			Ports:       "443",
		},
	}
)

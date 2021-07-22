package cloudfoundry

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
)

type SetupPhase interface {
	Run(logs io.Writer, home, name, source string) error

	WithBuildpacks(buildpacks ...string) SetupPhase
	WithEnv(env map[string]string) SetupPhase
	WithoutInternetAccess() SetupPhase
}

type Setup struct {
	cli  Executable
	home string

	internetAccess bool
	buildpacks     []string
	env            map[string]string
}

func NewSetup(cli Executable, home string) Setup {
	return Setup{
		cli:            cli,
		home:           home,
		internetAccess: true,
	}
}

func (s Setup) WithBuildpacks(buildpacks ...string) SetupPhase {
	s.buildpacks = buildpacks
	return s
}

func (s Setup) WithEnv(env map[string]string) SetupPhase {
	s.env = env
	return s
}

func (s Setup) WithoutInternetAccess() SetupPhase {
	s.internetAccess = false
	return s
}

func (s Setup) Run(log io.Writer, home, name, source string) error {
	err := os.MkdirAll(home, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to make temporary $CF_HOME: %w", err)
	}

	err = fs.Copy(s.home, filepath.Join(home, ".cf"))
	if err != nil {
		return fmt.Errorf("failed to copy $CF_HOME: %w", err)
	}

	env := append(os.Environ(), fmt.Sprintf("CF_HOME=%s", home))
	err = s.cli.Execute(pexec.Execution{
		Args:   []string{"create-org", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to create-org: %w\n\nOutput:\n%s", err, log)
	}

	err = s.cli.Execute(pexec.Execution{
		Args:   []string{"create-space", name, "-o", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to create-space: %w\n\nOutput:\n%s", err, log)
	}

	err = s.cli.Execute(pexec.Execution{
		Args:   []string{"target", "-o", name, "-s", name},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to target: %w\n\nOutput:\n%s", err, log)
	}

	configFile, err := os.Open(filepath.Join(home, ".cf", "config.json"))
	if err != nil {
		return err
	}
	defer configFile.Close()

	var config struct {
		Target string
	}
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		return err
	}

	target, err := url.Parse(config.Target)
	if err != nil {
		return err
	}

	securityGroup := PublicSecurityGroup
	if !s.internetAccess {
		securityGroup = PrivateSecurityGroup
	}

	addrs, err := net.LookupHost(target.Host)
	if err != nil {
		return err
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
		return err
	}

	err = os.WriteFile(filepath.Join(home, "security-group.json"), content, 0600)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(home, "empty-security-group.json"), []byte("[]"), 0600)
	if err != nil {
		return err
	}

	err = s.cli.Execute(pexec.Execution{
		Args:   []string{"create-security-group", name, filepath.Join(home, "security-group.json")},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to create-security-group: %w\n\nOutput:\n%s", err, log)
	}

	err = s.cli.Execute(pexec.Execution{
		Args:   []string{"bind-security-group", name, name, name, "--lifecycle", "staging"},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to bind-security-group: %w\n\nOutput:\n%s", err, log)
	}

	err = s.cli.Execute(pexec.Execution{
		Args:   []string{"update-security-group", "public_networks", filepath.Join(home, "empty-security-group.json")},
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to update-security-group: %w\n\nOutput:\n%s", err, log)
	}

	args := []string{"push", name, "-p", source, "--no-start"}
	for _, buildpack := range s.buildpacks {
		args = append(args, "-b", buildpack)
	}

	err = s.cli.Execute(pexec.Execution{
		Args:   args,
		Stdout: log,
		Stderr: log,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to push: %w\n\nOutput:\n%s", err, log)
	}

	var keys []string
	for key := range s.env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		err = s.cli.Execute(pexec.Execution{
			Args:   []string{"set-env", name, key, s.env[key]},
			Stdout: log,
			Stderr: log,
			Env:    env,
		})
		if err != nil {
			return fmt.Errorf("failed to set-env: %w\n\nOutput:\n%s", err, log)
		}
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

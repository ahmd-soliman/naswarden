package main

import (
	"fmt"
	"strings"

	"github.com/ahmd-soliman/naswarden/internal/docker"
)

// dockerAddress picks the Docker API address from the environment:
//
//   - DOCKER_PROXY_URL: a read-only socket proxy (http://, https://, tcp://).
//     naswarden's own variable, so it is the deliberate choice and wins.
//   - DOCKER_HOST: Docker's standard variable, for the mounted socket
//     (unix://) or a proxy (tcp://). It is also set for other tools, so it is
//     only used when DOCKER_PROXY_URL is not set.
//   - Neither set: the Docker socket at its conventional path, if one is
//     mounted there -- the same default as Docker's own CLI, and what most
//     catalog apps that mount the socket rely on. Mounting it is already the
//     explicit choice. variable is then "mounted socket".
//
// Nothing found: address is "" and Docker stays off. warning is non-empty
// when both variables are set to different values. err is set for a value
// naswarden cannot use; Docker then stays off too.
func dockerAddress(getenv func(string) string, isSocket func(string) bool) (variable, address, warning string, err error) {
	proxyURL, dockerHost := getenv("DOCKER_PROXY_URL"), getenv("DOCKER_HOST")
	switch {
	case proxyURL != "":
		variable, address = "DOCKER_PROXY_URL", proxyURL
		if dockerHost != "" && dockerHost != proxyURL {
			warning = "DOCKER_HOST is ignored because DOCKER_PROXY_URL is set"
		}
	case dockerHost != "":
		variable, address = "DOCKER_HOST", dockerHost
	case isSocket(docker.DefaultSocket):
		return "mounted socket", "unix://" + docker.DefaultSocket, "", nil
	default:
		return "", "", "", nil
	}

	if err := docker.ValidateHost(address); err != nil {
		return variable, address, warning, fmt.Errorf("invalid %s: %w", variable, err)
	}
	if variable == "DOCKER_PROXY_URL" && strings.HasPrefix(address, "unix://") {
		// The proxy variable names the read-only setup; a socket is not one,
		// so it has to be asked for by its own name.
		return variable, address, warning, fmt.Errorf("invalid DOCKER_PROXY_URL: %q is a Docker socket, not a proxy: set DOCKER_HOST for a socket", address)
	}
	return variable, address, warning, nil
}

// dockerStatus is what the UI shows about Docker: on (with how it is
// reached, for the footer), off (nothing configured or mounted, so the
// "Docker not connected" card explains how to turn it on), or error
// (configured but unusable, with the reason, e.g. a missing group).
// Access is "socket" (full access, nothing filters it), "proxy"
// (DOCKER_PROXY_URL) or "api" (DOCKER_HOST over the network, which may or
// may not be a filtering proxy).
type dockerStatus struct {
	State  string `json:"state"`
	Access string `json:"access,omitempty"`
	Error  string `json:"error,omitempty"`
}

func newDockerStatus(variable, address string, err error) dockerStatus {
	switch {
	case err != nil:
		return dockerStatus{State: "error", Error: err.Error()}
	case address == "":
		return dockerStatus{State: "off"}
	case strings.HasPrefix(address, "unix://"):
		return dockerStatus{State: "on", Access: "socket"}
	case variable == "DOCKER_PROXY_URL":
		return dockerStatus{State: "on", Access: "proxy"}
	default:
		return dockerStatus{State: "on", Access: "api"}
	}
}

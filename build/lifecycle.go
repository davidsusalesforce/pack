package build

import (
	"context"
	"fmt"
	"io"

	"github.com/buildpack/lifecycle/image/auth"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/buildpack/pack/containers"
	"github.com/buildpack/pack/logging"
)

type Lifecycle struct {
	BuilderImage    string
	Logger          *logging.Logger
	Context         context.Context
	Docker          Docker
	WorkspaceVolume string
}
type Phase struct {
	name     string
	logger   *logging.Logger
	context  context.Context
	docker   Docker
	ctrConf  *container.Config
	hostConf *container.HostConfig
	ctr      container.ContainerCreateCreatedBody
}

type Docker interface {
	RunContainer(ctx context.Context, id string, stdout io.Writer, stderr io.Writer) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
}

const (
	launchDir = "/workspace"
)

func (l *Lifecycle) NewPhase(name string, ops ...func(*Phase) (*Phase, error)) (*Phase, error) {
	ctrConf := &container.Config{
		Image:  l.BuilderImage,
		Labels: map[string]string{"author": "pack"},
	}
	hostConf := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:%s:", l.WorkspaceVolume, launchDir),
		},
	}
	ctrConf.Cmd = []string{"/lifecycle/" + name}
	step := &Phase{
		ctrConf:  ctrConf,
		hostConf: hostConf,
		name:     name,
		context:  l.Context,
		docker:   l.Docker,
		logger:   l.Logger,
	}
	var err error
	for _, op := range ops {
		step, err = op(step)
		if err != nil {
			return nil, err
		}
	}
	return step, nil
}

func (l *Lifecycle) WithArgs(args ...string) func(*Phase) (*Phase, error) {
	return func(phase *Phase) (*Phase, error) {
		phase.ctrConf.Cmd = append(phase.ctrConf.Cmd, args...)
		return phase, nil
	}
}

func (l *Lifecycle) WithDaemonAccess() func(*Phase) (*Phase, error) {
	return func(phase *Phase) (*Phase, error) {
		phase.ctrConf.User = "root"
		phase.hostConf.Binds = append(phase.hostConf.Binds, "/var/run/docker.sock:/var/run/docker.sock")
		return phase, nil
	}
}

func (l *Lifecycle) WithRegistryAccess(repos ...string) func(*Phase) (*Phase, error) {
	return func(phase *Phase) (*Phase, error) {
		authHeader, err := auth.BuildEnvVar(authn.DefaultKeychain, repos...)
		if err != nil {
			return nil, err
		}
		phase.ctrConf.Env = []string{fmt.Sprintf(`CNB_REGISTRY_AUTH=%s`, authHeader)}
		phase.hostConf.NetworkMode = "host"
		return phase, nil
	}
}

func (s *Phase) Run() error {
	var err error
	s.ctr, err = s.docker.ContainerCreate(s.context, s.ctrConf, s.hostConf, nil, "")
	if err != nil {
		return nil
	}
	return s.docker.RunContainer(
		s.context,
		s.ctr.ID,
		s.logger.VerboseWriter().WithPrefix(s.name),
		s.logger.VerboseErrorWriter().WithPrefix(s.name),
	)
}

func (s *Phase) Cleanup() error {
	return containers.Remove(s.docker, s.ctr.ID)
}

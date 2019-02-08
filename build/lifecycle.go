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
	"github.com/pkg/errors"

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
	files    []io.Reader
}

type Docker interface {
	RunContainer(ctx context.Context, id string, stdout io.Writer, stderr io.Writer) error
	CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options types.CopyToContainerOptions) error
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
		files:    []io.Reader{},
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

func WithArgs(args ...string) func(*Phase) (*Phase, error) {
	return func(phase *Phase) (*Phase, error) {
		phase.ctrConf.Cmd = append(phase.ctrConf.Cmd, args...)
		return phase, nil
	}
}

func WithDaemonAccess() func(*Phase) (*Phase, error) {
	return func(phase *Phase) (*Phase, error) {
		phase.ctrConf.User = "root"
		phase.hostConf.Binds = append(phase.hostConf.Binds, "/var/run/docker.sock:/var/run/docker.sock")
		return phase, nil
	}
}

func WithRegistryAccess(repos ...string) func(*Phase) (*Phase, error) {
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

func WithFiles(reader io.Reader) func(*Phase) (*Phase, error) {
	return func(phase *Phase) (*Phase, error) {
		phase.files = append(phase.files, reader)
		return phase, nil
	}
}

func (p *Phase) Run() error {
	var err error
	p.ctr, err = p.docker.ContainerCreate(p.context, p.ctrConf, p.hostConf, nil, "")
	if err != nil {
		return errors.Wrapf(err, "failed to create '%s' container", p.name)
	}
	for _, r := range p.files {
		if err := p.docker.CopyToContainer(p.context, p.ctr.ID, "/", r, types.CopyToContainerOptions{}); err != nil {
			return errors.Wrapf(err, "failed to copy files to '%s' container", p.name)
		}
	}
	return p.docker.RunContainer(
		p.context,
		p.ctr.ID,
		p.logger.VerboseWriter().WithPrefix(p.name),
		p.logger.VerboseErrorWriter().WithPrefix(p.name),
	)
}

func (p *Phase) Cleanup() error {
	return containers.Remove(p.docker, p.ctr.ID)
}

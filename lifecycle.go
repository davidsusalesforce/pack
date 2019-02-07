package pack

import (
	"context"

	"github.com/docker/docker/api/types/container"

	"github.com/buildpack/pack/logging"
)

type Lifecycle struct {
	BuilderImage string
	Logger       *logging.Logger
	Context      context.Context
	Docker       Docker
}
type LifecycleStep struct {
	name    string
	logger  *logging.Logger
	context context.Context
	docker  Docker
	conf    *container.Config
	ctr     container.ContainerCreateCreatedBody
}

func (l *Lifecycle) NewStep(name string, ops ...func(*LifecycleStep) *LifecycleStep) (*LifecycleStep, error) {
	ctrConf := &container.Config{
		Image: l.BuilderImage,
	}
	ctrConf.Cmd = []string{"/lifecycle/" + name}
	step := &LifecycleStep{
		conf:    ctrConf,
		name:    name,
		context: l.Context,
		docker:  l.Docker,
		logger:  l.Logger,
	}
	for _, op := range ops {
		step = op(step)
	}
	return step, nil
}

func (l *Lifecycle) WithArgs(args ...string) func(*LifecycleStep) *LifecycleStep {
	return func(step *LifecycleStep) *LifecycleStep {
		step.conf.Cmd = append(step.conf.Cmd, args...)
		return step
	}
}

func (s *LifecycleStep) Run() error {
	var err error
	s.ctr, err = s.docker.ContainerCreate(s.context, s.conf, &container.HostConfig{}, nil, "")
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

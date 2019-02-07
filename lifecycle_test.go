package pack_test

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/fatih/color"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpack/pack"
	"github.com/buildpack/pack/docker"
	"github.com/buildpack/pack/fs"
	"github.com/buildpack/pack/logging"
	h "github.com/buildpack/pack/testhelpers"
)

var (
	repoName  string
	dockerCli *docker.Client
)

func TestLifecycle(t *testing.T) {
	color.NoColor = true
	rand.Seed(time.Now().UTC().UnixNano())
	var err error
	dockerCli, err = docker.New()
	h.AssertNil(t, err)
	repoName = "lifecycle.step." + h.RandString(10)
	CreateFakeLifecycleImage(t, dockerCli, repoName)
	defer h.DockerRmi(dockerCli, repoName)

	spec.Run(t, "lifecycle", testLifecycle, spec.Report(report.Terminal{}))
}

func testLifecycle(t *testing.T, when spec.G, it spec.S) {
	when("LifecycleStep", func() {
		var (
			lifecycle      *pack.Lifecycle
			outBuf, errBuf bytes.Buffer
			logger         *logging.Logger
		)

		it.Before(func() {
			logger = logging.NewLogger(&outBuf, &errBuf, true, false)
			lifecycle = &pack.Lifecycle{
				BuilderImage: repoName,
				Logger:       logger,
				Context:      context.TODO(),
				Docker:       dockerCli,
			}
		})

		when("#Run", func() {
			it("runs the lifecycle step on the builder image", func() {
				step, err := lifecycle.NewStep("step")
				h.AssertNil(t, err)
				h.AssertNil(t, step.Run())
				h.AssertContains(t, outBuf.String(), "running some-lifecycle-step")
			})

			it("prefixes the output with the step name", func() {
				step, err := lifecycle.NewStep("step")
				h.AssertNil(t, err)
				h.AssertNil(t, step.Run())
				h.AssertContains(t, outBuf.String(), "[step] running some-lifecycle-step")
			})

			when("#WithArgs", func() {
				it("runs the lifecycle step with args", func() {
					step, err := lifecycle.NewStep("step", lifecycle.WithArgs("some", "args"))
					h.AssertNil(t, err)
					h.AssertNil(t, step.Run())
					h.AssertContains(t, outBuf.String(), `received args [/lifecycle/step some args]`)
				})
			})
		})
	})
}

func CreateFakeLifecycleImage(t *testing.T, dockerCli *docker.Client, repoName string) {
	ctx := context.Background()

	wd, err := os.Getwd()
	h.AssertNil(t, err)
	buildContext, _ := (&fs.FS{}).CreateTarReader(filepath.Join(wd, "testdata", "lifecycle"), "/", 0, 0)

	res, err := dockerCli.ImageBuild(ctx, buildContext, dockertypes.ImageBuildOptions{
		Tags:           []string{repoName},
		SuppressOutput: true,
		Remove:         true,
		ForceRemove:    true,
	})
	h.AssertNil(t, err)

	io.Copy(os.Stdout, res.Body)
	res.Body.Close()
}

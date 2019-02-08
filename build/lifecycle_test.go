package build_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/fatih/color"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpack/pack/build"
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
	repoName = "lifecycle.test." + h.RandString(10)
	CreateFakeLifecycleImage(t, dockerCli, repoName)
	defer h.DockerRmi(dockerCli, repoName)

	spec.Run(t, "lifecycle", testLifecycle, spec.Report(report.Terminal{}))
}

func testLifecycle(t *testing.T, when spec.G, it spec.S) {
	when("Phase", func() {
		var (
			lifecycle      *build.Lifecycle
			outBuf, errBuf bytes.Buffer
			logger         *logging.Logger
			volumeName     string
		)

		it.Before(func() {
			volumeName = "lifecycle.test" + h.RandString(10)
			logger = logging.NewLogger(&outBuf, &errBuf, true, false)
			lifecycle = &build.Lifecycle{
				BuilderImage:    repoName,
				WorkspaceVolume: volumeName,
				Logger:          logger,
				Context:         context.TODO(),
				Docker:          dockerCli,
			}
		})

		it.After(func() {
			h.AssertNil(t, dockerCli.VolumeRemove(context.TODO(), volumeName, true))
		})

		when("#Run", func() {
			it("runs the lifecycle phase on the builder image", func() {
				phase, err := lifecycle.NewPhase("phase")
				h.AssertNil(t, err)
				assertRunSucceeds(t, phase, &outBuf, &errBuf)
				h.AssertContains(t, outBuf.String(), "running some-lifecycle-phase")
			})

			it("prefixes the output with the phase name", func() {
				phase, err := lifecycle.NewPhase("phase")
				h.AssertNil(t, err)
				assertRunSucceeds(t, phase, &outBuf, &errBuf)
				h.AssertContains(t, outBuf.String(), "[phase] running some-lifecycle-phase")
			})

			it("attaches the workspace volume to /workspace", func() {
				phase, err := lifecycle.NewPhase("phase", build.WithArgs("workspace"))
				h.AssertNil(t, err)
				assertRunSucceeds(t, phase, &outBuf, &errBuf)
				h.AssertContains(t, outBuf.String(), "[phase] workspace test")
				txt := h.ReadFromDocker(t, volumeName, "/workspace/test.txt")
				h.AssertEq(t, txt, "test-workspace")
			})

			when("#WithArgs", func() {
				it("runs the lifecycle phase with args", func() {
					phase, err := lifecycle.NewPhase("phase", build.WithArgs("some", "args"))
					h.AssertNil(t, err)
					assertRunSucceeds(t, phase, &outBuf, &errBuf)
					h.AssertContains(t, outBuf.String(), `received args [/lifecycle/phase some args]`)
				})
			})

			when("#WithDaemonAccess", func() {
				it("allows daemon access inside the container", func() {
					phase, err := lifecycle.NewPhase(
						"phase",
						build.WithArgs("daemon"),
						build.WithDaemonAccess(),
					)
					h.AssertNil(t, err)
					assertRunSucceeds(t, phase, &outBuf, &errBuf)
					h.AssertContains(t, outBuf.String(), "[phase] daemon test")
				})
			})

			when("#WithRegistryAccess", func() {
				var registry *h.TestRegistryConfig

				it.Before(func() {
					registry = h.RunRegistry(t, true)
				})

				it.After(func() {
					registry.StopRegistry(t)
				})

				it("provides auth for registry in the container", func() {
					phase, err := lifecycle.NewPhase(
						"phase",
						build.WithArgs("registry", registry.RepoName("packs/build:v3alpha2")),
						build.WithRegistryAccess(),
					)
					h.AssertNil(t, err)
					assertRunSucceeds(t, phase, &outBuf, &errBuf)
					h.AssertContains(t, outBuf.String(), "[phase] registry test")
				})
			})

			when("#WithFiles", func() {
				it("copies the files into the container before running", func() {
					reader, err := (&fs.FS{}).CreateSingleFileTar("dummy/file/location/in/container", "some contents")
					h.AssertNil(t, err)
					phase, err := lifecycle.NewPhase(
						"phase",
						build.WithArgs("files", "/dummy/file/location/in/container"),
						build.WithFiles(reader),
					)
					h.AssertNil(t, err)
					assertRunSucceeds(t, phase, &outBuf, &errBuf)
					h.AssertContains(t, outBuf.String(), "[phase] file contents: some contents")
				})
			})
		})
	})
}

func assertRunSucceeds(t *testing.T, phase *build.Phase, outBuf *bytes.Buffer, errBuf *bytes.Buffer) {
	if err := phase.Run(); err != nil {
		phase.Cleanup()
		t.Fatalf("Failed to run phase '%s' \n stdout: '%s' \n stderr '%s'", err, outBuf.String(), errBuf.String())
	}
	phase.Cleanup()
}

func CreateFakeLifecycleImage(t *testing.T, dockerCli *docker.Client, repoName string) {
	ctx := context.Background()

	wd, err := os.Getwd()
	h.AssertNil(t, err)
	buildContext, _ := (&fs.FS{}).CreateTarReader(filepath.Join(wd, "testdata", "fake-lifecycle"), "/", 0, 0)

	res, err := dockerCli.ImageBuild(ctx, buildContext, dockertypes.ImageBuildOptions{
		Tags:        []string{repoName},
		Remove:      true,
		ForceRemove: true,
	})
	h.AssertNil(t, err)

	io.Copy(ioutil.Discard, res.Body)
	res.Body.Close()
}

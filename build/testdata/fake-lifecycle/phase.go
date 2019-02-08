package main

import (
	"context"
	"fmt"
	"os"

	"github.com/buildpack/lifecycle/image/auth"
	"github.com/docker/docker/api/types"
	dockercli "github.com/docker/docker/client"
	v1remote "github.com/google/go-containerregistry/pkg/v1/remote"
)

func main() {
	fmt.Println("running some-lifecycle-phase")
	fmt.Printf("received args %+v\n", os.Args)
	if len(os.Args) > 1 && os.Args[1] == "workspace" {
		testWorkspace()
	}
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		testDaemon()
	}
	if len(os.Args) > 1 && os.Args[1] == "registry" {
		testRegistryAccess(os.Args[2])
	}
}

func testWorkspace() {
	fmt.Println("workspace test")
	file, err := os.Create("/workspace/test.txt")
	if err != nil {
		fmt.Println("failed to create /workspace/test.txt")
		os.Exit(1)
	}
	defer file.Close()
	_, err = file.Write([]byte("test-workspace"))
	if err != nil {
		fmt.Println("failed to write to /workspace/test.txt")
		os.Exit(2)
	}
}

func testDaemon() {
	fmt.Println("daemon test")
	cli, err := dockercli.NewClientWithOpts(dockercli.FromEnv, dockercli.WithVersion("1.38"))
	if err != nil {
		fmt.Printf("failed to create new docker client: %s\n", err)
		os.Exit(3)
	}
	_, err = cli.ContainerList(context.TODO(), types.ContainerListOptions{})
	if err != nil {
		fmt.Printf("failed to access docker daemon: %s\n", err)
		os.Exit(4)
	}
}

func testRegistryAccess(repoName string) {
	fmt.Println("registry test")
	ref, auth, err := auth.ReferenceForRepoName(&auth.EnvKeychain{}, repoName)
	if err != nil {
		fmt.Println("fail")
		os.Exit(5)
	}
	_, err = v1remote.Image(ref, v1remote.WithAuth(auth))
	if err != nil {
		fmt.Println("failed to access image")
		os.Exit(6)
	}
}

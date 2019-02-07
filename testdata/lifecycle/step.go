package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("running some-lifecycle-step")
	fmt.Printf("received args %+v\n", os.Args)
}

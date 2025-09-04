package main

import (
	"fmt"
	"github.com/javanhut/Ivaldi-vcs/cli"
)

func main() {
	print := fmt.Println
	print("Ivaldi VCS")
	cli.Execute()
}

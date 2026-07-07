package main

import "github.com/MBarc/hush/internal/cli"

const version = "0.1.0-dev"

func main() {
	cli.Execute(version)
}

package main

import (
	"os"

	"github.com/getlawrence/cli/cmd"
)

func main() {
	if err := cmd.Execute(EmbeddedKnowledgeDB); err != nil {
		os.Exit(1)
	}
}

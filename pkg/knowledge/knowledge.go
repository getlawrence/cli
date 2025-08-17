package knowledge

import (
	"os"
)

type knowledge struct {
	data []byte
}

func NewKnowledge() *knowledge {
	data, err := os.ReadFile("otel_packages.json")
	if err != nil {
		panic(err)
	}
	return &knowledge{data: data}
}

var flow = `
user runs a command to analyze a codebase

the codebase is analyzed and the following information is collected:
- language
- dependencies
- frameworks
- libraries
- tools

then we need to find the best matching package for the codebase
based on the following criteria:
- codeing language (for example: javascript)
- runtime version (for example: node 18.19.0)

then we need to desiced otel core version to install and the matching exprimental version
then identify the instrumentation packages to install.
the goal of this knoladge base is to store all this information and provide an easy to use inrteface to accsess it.

we might accses it in many ways.


`

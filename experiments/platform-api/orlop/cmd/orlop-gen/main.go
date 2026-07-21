package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/thetechnick/orlop/pkg/generator"
)

func main() {
	var (
		inputDir  string
		outputDir string
	)

	flag.StringVar(&inputDir, "input-dir", "apis/private", "input directory containing private APIs")
	flag.StringVar(&outputDir, "output-dir", "apis/public", "output directory for public APIs")
	flag.Parse()

	gen, err := generator.NewGenerator(inputDir, outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := gen.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

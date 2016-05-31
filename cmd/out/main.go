package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/robdimsdale/concourse-pipeline-resource/cmd/out/filereader"
	"github.com/robdimsdale/concourse-pipeline-resource/concourse"
	"github.com/robdimsdale/concourse-pipeline-resource/concourse/api"
	"github.com/robdimsdale/concourse-pipeline-resource/fly"
	"github.com/robdimsdale/concourse-pipeline-resource/logger"
	"github.com/robdimsdale/concourse-pipeline-resource/out"
	"github.com/robdimsdale/concourse-pipeline-resource/sanitizer"
	"github.com/robdimsdale/concourse-pipeline-resource/validator"
)

const (
	flyBinaryName        = "fly"
	atcExternalURLEnvKey = "ATC_EXTERNAL_URL"
)

var (
	// version is deliberately left uninitialized so it can be set at compile-time
	version string

	l logger.Logger
)

func main() {
	if version == "" {
		version = "dev"
	}

	if len(os.Args) < 2 {
		log.Fatalln(fmt.Sprintf(
			"not enough args - usage: %s <sources directory>", os.Args[0]))
	}

	sourcesDir := os.Args[1]

	outDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalln(err)
	}

	var input concourse.OutRequest

	logFile, err := ioutil.TempFile("", "concourse-pipeline-resource-out.log")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Fprintf(logFile, "Concourse Pipeline Resource version: %s\n", version)

	fmt.Fprintf(os.Stderr, "Logging to %s\n", logFile.Name())

	err = json.NewDecoder(os.Stdin).Decode(&input)
	if err != nil {
		fmt.Fprintf(logFile, "Exiting with error: %v\n", err)
		log.Fatalln(err)
	}

	sanitized := concourse.SanitizedSource(input.Source)
	sanitizer := sanitizer.NewSanitizer(sanitized, logFile)

	l = logger.NewLogger(sanitizer)

	flyBinaryPath := filepath.Join(outDir, flyBinaryName)
	flyConn := fly.NewFlyConn("concourse-pipeline-resource-target", l, flyBinaryPath)

	err = validator.ValidateOut(input)
	if err != nil {
		l.Debugf("Exiting with error: %v\n", err)
		log.Fatalln(err)
	}

	if input.Params.PipelinesFile != "" {
		pipelinesFromFile, err := filereader.PipelinesFromFile(input.Params.PipelinesFile, sourcesDir)
		if err != nil {
			l.Debugf("Exiting with error: %v\n", err)
			log.Fatalln(err)
		}

		input.Params.PipelinesFile = ""
		input.Params.Pipelines = pipelinesFromFile
	}

	// Validate contents of pipelines file
	err = validator.ValidateOut(input)
	if err != nil {
		l.Debugf("Exiting with error: %v\n", err)
		log.Fatalln(err)
	}

	if input.Source.Target == "" {
		input.Source.Target = os.Getenv(atcExternalURLEnvKey)
	}

	apiClient := api.NewClient(input.Source.Target, input.Source.Username, input.Source.Password)
	response, err := out.NewOutCommand(version, l, flyConn, apiClient, sourcesDir).Run(input)
	if err != nil {
		l.Debugf("Exiting with error: %v\n", err)
		log.Fatalln(err)
	}

	l.Debugf("Returning output: %+v\n", response)

	err = json.NewEncoder(os.Stdout).Encode(response)
	if err != nil {
		l.Debugf("Exiting with error: %v\n", err)
		log.Fatalln(err)
	}
}

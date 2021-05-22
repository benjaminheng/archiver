package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	inputDir  = flag.String("input", "", "Path to input directory")
	outputDir = flag.String("output", "", "Path to output directory")
)

// Metadata holds metadata about an archived resource.
type Metadata struct {
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	ArchivedAt time.Time `json:"archived_at"`
}

// ArchivedResource represents a webpage that has been processed by some form
// of readability plugin. A common reference implementation of a readability
// plugin is https://github.com/mozilla/readability.
type ArchivedResource struct {
	URL      string
	Title    string
	HTMLBody string
}

type Archiver struct {
	InputDir  string
	OutputDir string
}

func (a *Archiver) processLinksInMarkdownFile(filePath string) {
	links := a.parseLinks(filePath)
	if len(links) > 0 {
		for _, url := range links {
			// TODO: check if output path exists to see if it's already been written
			// TODO: process with readability
			archivedResource := ArchivedResource{
				URL:      url,
				Title:    "", // TODO
				HTMLBody: "", // TODO
			}
			_ = archivedResource
			metadata := Metadata{
				URL:        url,
				Title:      "", // TODO
				ArchivedAt: time.Now(),
			}
			_ = metadata
			// TODO: write content + metadata to output dir
			// TODO: interact with cache here to avoid duplicate writes
		}
	}
}

func (a *Archiver) parseLinks(filePath string) []string {
	return nil
}

func (a *Archiver) Archive() error {
	err := filepath.Walk(a.InputDir,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				// TODO: directory, recurse
			}
			if strings.HasSuffix(filePath, ".md") || strings.HasSuffix(filePath, ".markdown") {
				a.processLinksInMarkdownFile(filePath)
			}
			return nil
		})
	if err != nil {
		return err
	}
	return nil
}

func validateArgs() error {
	if *inputDir == "" && *outputDir == "" {
		return errors.New("input and output directory must be specified")
	}
	// TODO: check if directories exist
	return nil
}

func main() {
	flag.Parse()

	if err := validateArgs(); err != nil {
		log.Fatal(err)
	}

	archiver := Archiver{
		InputDir:  *inputDir,
		OutputDir: *outputDir,
	}
	archiver.Archive()
}

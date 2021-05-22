package main

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// NOTE: regex has an edge case where it won't match a string starting with a
// valid link. Must have at least one character between the start of line and
// the link.
//
// [^!]                                 - Don't match if starts with `!` (link is an image)
//     \[[^][]+\]                       - 1+ occurances of non-][ character
//               \(                     - Opening brace containing the URL
//		   (https?://           - Capture group: http:// or https://
//                           [^()]+)    - 1+ characters of non-)( character. End of capture group
//                                  \)  - Closing brace containing the URL
var markdownLinkRegex = regexp.MustCompile(`[^!]\[[^][]+]\((https?://[^()]+)\)`)

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

func (a *Archiver) processLinksInMarkdownFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	links, err := parseLinksFromMarkdown(string(b))
	if err != nil {
		return err
	}
	if len(links) > 0 {
		for _, link := range links {
			linkID, err := getLinkID(link)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot get link ID: %v", err)
			}

			// check if link has been archived before
			destinationPath := path.Join(*outputDir, linkID)
			_, err = os.Stat(destinationPath)
			if !os.IsNotExist(err) {
				continue
			}

			fmt.Printf("destinationPath = %+v\n", destinationPath)
			// TODO: check if output path exists to see if it's already been written
			// TODO: process with readability
			archivedResource := ArchivedResource{
				URL:      link,
				Title:    "", // TODO
				HTMLBody: "", // TODO
			}
			_ = archivedResource
			metadata := Metadata{
				URL:        link,
				Title:      "", // TODO
				ArchivedAt: time.Now(),
			}
			_ = metadata
			// TODO: write content + metadata to output dir
			// TODO: interact with cache here to avoid duplicate writes
		}
	}
	return nil
}

func getLinkID(link string) (string, error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}

	// link ID before processing
	linkID := fmt.Sprintf("%s_%s", u.Host, u.RequestURI())

	// processing:
	// 1. replace / with _
	// 2. replace ?= with -
	// 3. remove any character not in our allowed set
	linkID = strings.ReplaceAll(linkID, "/", "_")
	linkID = strings.ReplaceAll(linkID, "?", "-")
	linkID = strings.ReplaceAll(linkID, "=", "-")
	r := regexp.MustCompile("[^a-zA-Z0-9_?=.-]+")
	linkID = r.ReplaceAllString(linkID, "")
	linkID = strings.TrimRight(linkID, "_")

	// truncate link ID
	runes := []rune(linkID)
	if len(runes) > 100 {
		linkID = string(runes[:100])
	}

	// append a hash for uniqueness
	hash := sha256.Sum256([]byte(linkID))
	truncatedHash := fmt.Sprintf("%x", hash)[:8]
	linkID = linkID + "_" + truncatedHash

	return linkID, nil
}

func parseLinksFromMarkdown(markdown string) (links []string, err error) {
	matches := markdownLinkRegex.FindAllStringSubmatch(markdown, -1)
	for _, match := range matches {
		links = append(links, match[1])
	}
	return links, nil
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
				err := a.processLinksInMarkdownFile(filePath)
				if err != nil {
					return err
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	return nil
}

func validateArgs() error {
	if *inputDir == "" || *outputDir == "" {
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

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
	"sort"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"gopkg.in/yaml.v2"
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
	URL        string    `yaml:"url"`
	Title      string    `yaml:"title"`
	ArchivedAt time.Time `yaml:"archived_at"`
}

type Archiver struct {
	InputDir  string
	OutputDir string

	checkedLinks map[string]bool
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

			if a.isLinkCheckedBefore(linkID) {
				continue
			}

			// check if link has been archived before
			linkIDFilePath := path.Join(*outputDir, linkID)
			_, err = os.Stat(linkIDFilePath)
			if !os.IsNotExist(err) {
				// cache file is out of sync with directory structure, update cache
				a.setLinkChecked(linkID)
				continue
			}

			// apply readability
			article, err := readability.FromURL(link, 5*time.Second)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot apply readability for %+v: %+v\n", link, err)
				a.setLinkChecked(linkID)
				continue
			}

			// construct archived file contents
			metadata := Metadata{
				URL:        link,
				Title:      article.Title,
				ArchivedAt: time.Now(),
			}
			b, err := yaml.Marshal(metadata)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot marshal yaml frontmatter for %+v: %+v\n", link, err)
				continue
			}
			content := fmt.Sprintf("---\n%s\n---\n%s", strings.Trim(string(b), "\n"), article.Content)

			// write content to file
			err = os.Mkdir(linkIDFilePath, 0755)
			if err != nil {
				return err
			}
			archivedFile, err := os.Create(path.Join(linkIDFilePath, "index.html"))
			if err != nil {
				return err
			}
			archivedFile.WriteString(content)
			archivedFile.Close()

			fmt.Printf("Archived %s\n", link)
			a.setLinkChecked(linkID)
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
	err := a.initCheckedLinkCache()
	if err != nil {
		return err
	}
	err = filepath.Walk(a.InputDir,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
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
	err = a.writeCheckedLinkCache()
	if err != nil {
		return err
	}
	return nil
}

func (a *Archiver) setLinkChecked(linkID string) {
	if a.checkedLinks != nil {
		a.checkedLinks[linkID] = true
	}
}

func (a *Archiver) writeCheckedLinkCache() error {
	cacheFile, err := os.OpenFile(path.Join(*outputDir, ".checked_links.txt"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer cacheFile.Close()
	checkedLinks := make([]string, 0, len(a.checkedLinks))
	for v := range a.checkedLinks {
		checkedLinks = append(checkedLinks, v)
	}
	sort.Strings(checkedLinks)
	cacheFile.WriteString(strings.Join(checkedLinks, "\n"))
	return nil
}

func (a *Archiver) initCheckedLinkCache() error {
	if a.checkedLinks == nil {
		cacheFile, err := os.OpenFile(path.Join(*outputDir, ".checked_links.txt"), os.O_CREATE|os.O_RDONLY, 0644)
		if err != nil {
			return err
		}
		defer cacheFile.Close()
		b, err := io.ReadAll(cacheFile)
		if err != nil {
			return err
		}
		links := strings.Split(string(b), "\n")
		a.checkedLinks = make(map[string]bool)
		for _, v := range links {
			a.checkedLinks[v] = true
		}
	}
	return nil
}

func (a *Archiver) isLinkCheckedBefore(linkID string) bool {
	return a.checkedLinks[linkID]
}

func validateArgs() error {
	if *inputDir == "" || *outputDir == "" {
		return errors.New("input and output directory must be specified")
	}
	fileInfo, err := os.Stat(*inputDir)
	if os.IsNotExist(err) {
		return errors.New("input does not exist")
	} else if !fileInfo.IsDir() {
		return errors.New("input is not a directory")
	}
	fileInfo, err = os.Stat(*outputDir)
	if os.IsNotExist(err) {
		return errors.New("output does not exist")
	} else if !fileInfo.IsDir() {
		return errors.New("output is not a directory")
	}
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
	err := archiver.Archive()
	if err != nil {
		log.Fatal(err)
	}
}

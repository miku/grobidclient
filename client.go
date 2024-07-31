package grobidclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// ValidServices, see also: https://grobid.readthedocs.io/en/latest/Grobid-service/#grobid-web-services
var ValidServices = []string{
	"processFulltextDocument",
	"processHeaderDocument",
	"processReferences",
	"processCitationList",
	"processCitationPatentST36",
	"processCitationPatentPDF",
}

// IsValidService returns true, if the service name is valid.
func IsValidService(name string) bool {
	for _, v := range ValidServices {
		if v == name {
			return true
		}
	}
	return false
}

// ErrInvalidService, if the service name is not known.
var ErrInvalidService = errors.New("invalid service")

// Doer is a minimal, local HTTP client abstraction.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Grobid client, embedding an HTTP client for flexibility.
type Grobid struct {
	Server string
	Client Doer
}

// Options are grobid API options.
type Options struct {
	GenerateIDs            bool
	ConsolidateHeader      bool
	ConsolidateCitations   bool
	IncludeRawCitations    bool
	IncluseRawAffiliations bool
	TEICoordinates         bool
	SegmentSentences       bool
	Force                  bool
	Verbose                bool
	OutputDir              string
}

// Result is returned from ProcessText services, not necessarily successful.
type Result struct {
	Filename   string
	StatusCode int
	Body       []byte
}

// StringBody returns the response body as string.
func (r *Result) StringBody() string {
	return string(r.Body)
}

// String representation of a result.
func (r *Result) String() string {
	return fmt.Sprintf("%d on %s, body: %s", r.StatusCode, r.Filename, string(r.Body))
}

// writeFields writes set flags to a writer.
func (opts *Options) writeFields(w *multipart.Writer) {
	if opts.ConsolidateCitations {
		w.WriteField("consolidateCitations", "1")
	}
	if opts.ConsolidateHeader {
		w.WriteField("consolidateHeader", "1")
	}
	if opts.GenerateIDs {
		w.WriteField("generateIDs", "1")
	}
	if opts.IncludeRawCitations {
		w.WriteField("includeRawCitations", "1")
	}
	if opts.IncluseRawAffiliations {
		w.WriteField("includeRawAffiliations", "1")
	}
	if opts.SegmentSentences {
		w.WriteField("segmentSentences", "1")
	}
}

// Ping tests the server connection.
func (g *Grobid) Ping() error {
	u, err := url.JoinPath(g.Server, "api", "isalive")
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with: %v", http.StatusText(resp.StatusCode))
	}
	return nil
}

// Pingmoji returns an emoji version of ping.
func (g *Grobid) Pingmoji() string {
	if g.Ping() == nil {
		return "✅"
	}
	return "⛔"
}

func withoutExt(filepath string) string {
	return strings.TrimSuffix(filepath, path.Ext(filepath))
}

// outputFilename returns a suitable output filename. If dir is empty, the
// output is written in the same directory as the input file.
func outputFilename(filepath, dir string) string {
	const ext = ".grobid.tei.xml"
	if dir == "" {
		return withoutExt(filepath) + ext
	} else {
		return path.Join(dir, withoutExt(path.Base(filepath))+ext)
	}
}

// ProcessDirRecursive takes a directory name, finds all files that look like
// PDF files and processes them. TODO: also process select text files, and
// patents; also we should use context for cancellation here.
func (g *Grobid) ProcessDirRecursive(dir, service string, numWorkers int, opts *Options) error {
	var (
		pathC   = make(chan string)
		resultC = make(chan *Result)
		errC    = make(chan error)
		wg      sync.WaitGroup
		done    = make(chan bool)
	)
	if opts.OutputDir != "" {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return err
		}
	}
	// start N workers, TODO: cancellation
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathC {
				name := outputFilename(path, opts.OutputDir)
				if _, err := os.Stat(name); err == nil && !opts.Force {
					log.Printf("already processed: %v", name)
					continue
				}
				result, err := g.ProcessPDF(path, service, opts)
				if err != nil {
					errC <- err
					break
				}
				resultC <- result
			}
		}()
	}
	// TODO: actually do something with the result, e.g. write them to a
	// adjacent file.
	resultWorker := func() {
		for result := range resultC {
			name := outputFilename(result.Filename, opts.OutputDir)
			if err := os.WriteFile(name, result.Body, 0644); err != nil {
				log.Println(err)
			}
			log.Printf("got result [%d]: %v", result.StatusCode, result.Filename)
		}
		done <- true
	}
	go resultWorker()
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !isPDF(path) {
			return nil
		}
		select {
		case pathC <- path:
		case err := <-errC:
			return err
		}
		return nil
	})
	close(pathC)
	wg.Wait()
	close(resultC)
	<-done
	return err
}

func isPDF(filename string) bool {
	// TODO: could also do some content type snooping.
	return strings.HasSuffix(strings.ToLower(filename), ".pdf")
}

// ProcessPDF processes a single PDF with given options. Result contains the
// HTTP status code, indicating success or failure.
func (g *Grobid) ProcessPDF(filename, service string, opts *Options) (*Result, error) {
	if !IsValidService(service) {
		return nil, ErrInvalidService
	}
	serviceURL, err := url.JoinPath(g.Server, "api", service)
	if err != nil {
		return nil, err
	}
	var (
		pr, pw = io.Pipe()
		mw     = multipart.NewWriter(pw)
		errC   = make(chan error)
	)
	go func() {
		f, err := os.Open(filename)
		if err != nil {
			errC <- err
		}
		defer f.Close()
		opts.writeFields(mw)
		part, err := mw.CreateFormFile("input", filepath.Base(filename))
		if err != nil {
			errC <- err
		}
		if _, err := io.Copy(part, f); err != nil {
			errC <- err
		}
		if err := mw.Close(); err != nil {
			errC <- err
		}
		if err := pw.Close(); err != nil {
			errC <- err
		}
		errC <- nil
	}()
	req, err := http.NewRequest("POST", serviceURL, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/xml")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if err := <-errC; err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Filename:   filename,
		StatusCode: resp.StatusCode,
		Body:       b,
	}
	return result, nil
}

// ProcessText processes a single text file with given options.
func (g *Grobid) ProcessText(filename, service string, opts *Options) (*Result, error) {
	if !IsValidService(service) {
		return nil, ErrInvalidService
	}
	serviceURL, err := url.JoinPath(g.Server, "api", service)
	if err != nil {
		return nil, err
	}
	var (
		buf     bytes.Buffer
		enc     = json.NewEncoder(&buf)
		payload struct {
			ConsolidateCitations string   `json:"consolidateCitations,omitempty"`
			ConsolidateHeader    string   `json:"consolidateHeader,omitempty"`
			Citations            []string `json:"citations"`
		}
	)
	if err := parseLines(filename, payload.Citations); err != nil {
		return nil, err
	}
	if opts.ConsolidateCitations {
		payload.ConsolidateCitations = "1"
	}
	if opts.ConsolidateHeader {
		payload.ConsolidateHeader = "1"
	}
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", serviceURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Filename:   filename,
		StatusCode: resp.StatusCode,
		Body:       b,
	}
	return result, nil
}

// parseLines reads lines in a file into a given string slice.
func parseLines(filename string, lines []string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return nil
}
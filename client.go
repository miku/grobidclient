package grobidclient

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sethgrid/pester"
)

// Version of grobidclient.
var Version = "0.2.2"

// ErrInvalidService, if the service name is not known.
var ErrInvalidService = errors.New("invalid service")

// DefaultExt for structured metadata outputs.
const DefaultExt = "grobid.tei.xml"

// ValidServices, see also:
// https://grobid.readthedocs.io/en/latest/Grobid-service/#grobid-web-services
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

// DefaultOptions to send to GROBID.
var DefaultOptions = &Options{
	GenerateIDs:            true,
	ConsolidateHeader:      true,
	ConsolidateCitations:   true,
	IncludeRawCitations:    true,
	IncluseRawAffiliations: true,
	TEICoordinates:         []string{"ref", "figure", "persName", "formula", "biblStruct"},
	SegmentSentences:       true,
	Force:                  false,
	Verbose:                false,
	OutputDir:              "",
	CreateHashSymlinks:     false,
}

// Options are grobid API options. Full documentation can be found at
// https://grobid.readthedocs.io/en/latest/Grobid-service/#grobid-web-services.
type Options struct {
	GenerateIDs            bool
	ConsolidateHeader      bool
	ConsolidateCitations   bool
	IncludeRawCitations    bool
	IncluseRawAffiliations bool
	TEICoordinates         []string // https://grobid.readthedocs.io/en/latest/Coordinates-in-PDF/
	SegmentSentences       bool
	Force                  bool
	Verbose                bool
	OutputDir              string
	CreateHashSymlinks     bool
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
	for _, v := range opts.TEICoordinates {
		w.WriteField("teiCoordinates", v)
	}
}

// Result wraps a server response, not necessarily successful. If processing
// failed, Err will contain the first error encountered.
type Result struct {
	Filename       string
	SHA1Hex        string
	StatusCode     int
	Body           []byte
	Err            error
	ProcessingTime time.Duration
}

// StringBody returns the response body as string.
func (r *Result) StringBody() string {
	return string(r.Body)
}

// String representation of a result.
func (r *Result) String() string {
	return fmt.Sprintf("%d on %s, body: %s", r.StatusCode, r.Filename, string(r.Body))
}

// Doer is a minimal, local HTTP client abstraction.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// New creates a new Grobid client with a recommended, resilient HTTP client.
func New(server string) *Grobid {
	hc := &http.Client{
		Timeout: 60 * time.Second,
	}
	client := pester.NewExtendedClient(hc)
	client.MaxRetries = 3
	client.Backoff = pester.ExponentialBackoff
	client.RetryOnHTTP429 = true
	return &Grobid{
		Server: server,
		Client: client,
	}
}

// Grobid client, embedding an HTTP client for flexibility.
type Grobid struct {
	Server string
	Client Doer
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

// Pingmoji returns an emoji rendering of a ping result.
func (g *Grobid) Pingmoji() string {
	if g.Ping() == nil {
		return "✅"
	}
	return "⛔"
}

// withoutExt returns the given file or path without the extension.
func withoutExt(filepath string) string {
	return strings.TrimSuffix(filepath, path.Ext(filepath))
}

// outputFilename returns a suitable output filename. If dir is empty, the
// output is written in the same directory as the input file.
func outputFilename(filepath string, opts *Options) string {
	if opts.OutputDir == "" {
		return withoutExt(filepath) + "." + DefaultExt
	} else {
		return path.Join(opts.OutputDir, withoutExt(path.Base(filepath))+"."+DefaultExt)
	}
}

// isAlreadyProcessed returns true, if the file at a given path has been
// processed. Note: this does not work with hash based naming as for those the
// file contents needs to be completely read already. This should be a fast
// operation.
func (g *Grobid) isAlreadyProcessed(path string, opts *Options) bool {
	name := outputFilename(path, opts)
	_, err := os.Stat(name)
	return err == nil
}

// ResultFunc is a function invoked on the result of the processing.
type ResultFunc func(*Result, *Options) error

// DebugResultWriter is a dummy result writer, which only logs the result.
func DebugResultWriter(result *Result, _ *Options) error {
	if result.Err != nil {
		log.Printf("[%d][%s] %s [%v][%v]",
			result.StatusCode, result.SHA1Hex, result.Filename, result.ProcessingTime, result.Err)
	} else {
		log.Printf("[%d][%s] %s [%v][]",
			result.StatusCode, result.SHA1Hex, result.Filename, result.ProcessingTime)
	}
	return result.Err
}

// DefaultResultWriter is a ResultFunc that writes out a single file with the
// result. It contains handling to write out error results akin to the Python
// grobid client library.
func DefaultResultWriter(result *Result, opts *Options) error {
	if opts == nil {
		opts = DefaultOptions
	}
	if result == nil || reflect.DeepEqual(result, &Result{}) {
		return nil
	}
	dst := outputFilename(result.Filename, opts)
	if err := os.MkdirAll(path.Dir(dst), 0755); err != nil {
		return err
	}
	if result.StatusCode != 200 || len(result.Body) == 0 {
		// writing error file with suffixed error code
		dst = strings.Replace(dst, "."+DefaultExt, fmt.Sprintf("_%d.txt", result.StatusCode), 1)
		return os.WriteFile(dst, result.Body, 0644)
	}
	if opts.Verbose {
		log.Printf("done: %s", dst)
	}
	// write TEI file
	err := os.WriteFile(dst, result.Body, 0644)
	if err != nil {
		return err
	}
	if opts.CreateHashSymlinks {
		link := path.Join(path.Dir(dst), fmt.Sprintf("%s.%s", result.SHA1Hex, DefaultExt))
		if err := os.Symlink(path.Base(dst), link); err != nil {
			return err
		}
	}
	return nil
}

// ProcessDirRecursive recursively walks a given directory "dir" and run
// parsing using "service" on each file. A number of workers can be started and
// a ResultFunc can be specified, which gets called for each result, e.g. to
// write debug output to stderr or to write a file with the structured metadata
// to disk. Options contain options to be passed to GROBID API, using defaults
// if they are not set.
func (g *Grobid) ProcessDirRecursive(dir, service string, numWorkers int, rf ResultFunc, opts *Options) error {
	var (
		pathC        = make(chan string)
		errC         = make(chan error)
		done         = make(chan bool)
		wg           sync.WaitGroup
		errList      []error
		numProcessed int
	)
	if opts == nil {
		opts = DefaultOptions
	}
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathC {
				if g.isAlreadyProcessed(path, opts) && !opts.Force {
					log.Printf("already processed: %s", path)
					continue
				}
				var (
					result *Result
					err    error
				)
				switch {
				case service == "processCitationList":
					result, err = g.ProcessText(path, service, opts)
				default:
					result, err = g.ProcessPDF(path, service, opts)
				}
				if result == nil {
					result = &Result{
						// If processing failed, return a pseudo-result
						// nonetheless, so we still know know about the error
						// conditions.
						Filename:   path,
						StatusCode: -1,
						Err:        fmt.Errorf("process failed: %w", err),
					}
				}
				errC <- rf(result, opts)
			}
		}()
	}
	go func() {
		for err := range errC {
			if err == nil {
				continue
			}
			// aggregate errors in error list
			errList = append(errList, err)
		}
		done <- true
	}()
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Note: Following the Python client, which has hardcoded rules for
		// what service and what filetype fit together.
		switch {
		case service == "processFulltextDocument" && isPDF(path):
			if opts.Verbose {
				log.Printf("enqueued: %s", path)
			}
			pathC <- path
		case service == "processCitationList" && isText(path):
			if opts.Verbose {
				log.Printf("enqueued: %s", path)
			}
			pathC <- path
		case service == "processCitationPatentST36" && isXML(path):
			if opts.Verbose {
				log.Printf("enqueued: %s", path)
			}
			pathC <- path
		default:
			if opts.Verbose {
				log.Printf("skipping: %s", path)
			}
			return nil
		}
		numProcessed++
		return nil
	})
	if err != nil {
		return err
	}
	close(pathC)
	wg.Wait()
	close(errC)
	<-done
	log.Printf("processed %d docs, with %d errors", numProcessed, len(errList))
	if len(errList) > 0 {
		return errors.Join(errList...)
	}
	return nil
}

// isPDF returns true, if the given file is likely a PDF.
func isPDF(filename string) bool {
	mtype, err := mimetype.DetectFile(filename)
	if err != nil {
		return strings.HasSuffix(strings.ToLower(filename), ".pdf")
	}
	return mtype.Is("application/pdf")
}

// isXML returns true, if the filename is likely an XML file.
func isXML(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".xml")
}

// isText returns true, if the filename is likely an text file.
func isText(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".txt")
}

// ProcessPDFContext analysis a single PDF, with cancellation options.
func (g *Grobid) ProcessPDFContext(ctx context.Context, filename, service string, opts *Options) (*Result, error) {
	var started = time.Now()
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, err
	}
	if opts == nil {
		opts = DefaultOptions
	}
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
		h      = sha1.New()
		errC   = make(chan error)
	)
	go func() {
		defer close(errC)
		f, err := os.Open(filename)
		if err != nil {
			errC <- err
			return
		}
		defer f.Close()
		opts.writeFields(mw)
		part, err := mw.CreateFormFile("input", filepath.Base(filename))
		if err != nil {
			errC <- err
			return
		}
		tee := io.TeeReader(f, h)
		if _, err := io.Copy(part, tee); err != nil {
			errC <- err
			return
		}
		if err := mw.Close(); err != nil {
			errC <- err
			return
		}
		if err := pw.Close(); err != nil {
			errC <- err
			return
		}
		errC <- nil
	}()
	req, err := http.NewRequestWithContext(ctx, "POST", serviceURL, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/xml")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// This works, because the copy goroutine returns exactly one value. If
	// there is an error in opening the file, we may not see this error. TODO:
	// test case.
	if err := <-errC; err != nil {
		return nil, err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Filename:       filename,
		Body:           b,
		SHA1Hex:        fmt.Sprintf("%x", h.Sum(nil)),
		StatusCode:     resp.StatusCode,
		ProcessingTime: time.Since(started),
	}
	return result, nil
}

// ProcessPDF processes a single PDF with given options. Result contains the
// HTTP status code, indicating success or failure.
func (g *Grobid) ProcessPDF(filename, service string, opts *Options) (*Result, error) {
	return g.ProcessPDFContext(context.Background(), filename, service, opts)
}

// ProcessText processes a single text file with given options.
func (g *Grobid) ProcessText(filename, service string, opts *Options) (*Result, error) {
	started := time.Now()
	if !IsValidService(service) {
		return nil, ErrInvalidService
	}
	if opts == nil {
		opts = DefaultOptions
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
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	lines, err := parseLines(f)
	if err != nil {
		return nil, err
	}
	payload.Citations = lines
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
		Filename:       filename,
		StatusCode:     resp.StatusCode,
		Body:           b,
		ProcessingTime: time.Since(started),
	}
	return result, nil
}

// parseLines reads lines in a file into a given string slice.
func parseLines(r io.Reader) (lines []string, err error) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return lines, err
		}
		v := strings.TrimSpace(line)
		if len(v) == 0 {
			continue
		}
		lines = append(lines, v)
	}
	return lines, nil
}

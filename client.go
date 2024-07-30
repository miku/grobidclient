package grobidclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ValidServices = []string{
	"processFulltextDocument",
	"processHeaderDocument",
	"processReferences",
	"processCitationList",
	"processCitationPatentST36",
	"processCitationPatentPDF",
}

func isValidService(name string) bool {
	for _, v := range ValidServices {
		if v == name {
			return true
		}
	}
	return false
}

var ErrInvalidService = errors.New("invalid service")

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

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
}

// Result is returned from ProcessText services.
type Result struct {
	Filename   string
	StatusCode int
	Body       []byte
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

func withoutExt(filepath string) string {
	return strings.TrimSuffix(filepath, path.Ext(filepath))
}

func outputFilename(filepath string) string {
	const ext = "grobid.tei.xml"
	var (
		dir = path.Dir(filepath)
		fn  = withoutExt(filepath) + ext
	)
	return path.Join(dir, fn)
}

// ProcessPDF processes a single PDF with given options.
func (g *Grobid) ProcessPDF(filename, service string, opts *Options) (*Result, error) {
	if !isValidService(service) {
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
	req.Header.Set("Accept", "text/plain")
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
	if !isValidService(service) {
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

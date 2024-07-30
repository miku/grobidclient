package grobidclient

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
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

type Options struct {
	Service                string
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

// Ping tests the server connection.
func (g *Grobid) Ping() error {
	u := url.JoinPath(g.Server, "api", "isalive")
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
	return filepath[:len(filepath)-len(path.Ext(filepath))]
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
func (g *Grobid) ProcessPDF(filename, service string, opts *Options) error {
	if !isValidService(service) {
		return ErrInvalidService
	}
	return nil
}

// ProcessText processes a single text file with given options.
func (g *Grobid) ProcessText(filename, service string, opts *Options) error {
	if !isValidService(service) {
		return ErrInvalidService
	}
	return nil
}

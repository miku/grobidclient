// https://github.com/allenai/s2orc-doc2json/blob/71c022ed4bed3ffc71d22c2ac5cdbc133ad04e3c/doc2json/grobid2json/tei_to_json.py#L691
package tei

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/beevik/etree"
)

const (
	XMLNS = "http://www.w3.org/XML/1998/namespace"
	NS    = "http://www.tei-c.org/ns/1.0"
)

var ErrInvalidDocument = errors.New("invalid document")

func ParseCitationList() {}
func ParseCitation()     {}
func ParseCitations()    {}
func ParseDocument(r io.Reader) error {
	tree := etree.NewDocument()
	_, err := tree.ReadFrom(r)
	if err != nil {
		return err
	}
	tei := tree.Root()
	header := tei.FindElement(fmt.Sprintf(".//teiHeader[namespace-uri()=%q]", NS))
	if header == nil {
		return ErrInvalidDocument
	}
	log.Println(header)
	applicationTags := header.FindElements(
		fmt.Sprintf(".//appInfo[namespace-uri()=%q]/application[namespace-uri()=%q]", NS, NS))
	if len(applicationTags) == 0 {
		return ErrInvalidDocument
	}
	var (
		applicationTag = applicationTags[0]
		version        = strings.TrimSpace(applicationTag.SelectAttr("version").Value)
		ts             = strings.TrimSpace(applicationTag.SelectAttr("when").Value)
	)
	doc := GrobidDocument{
		GrobidVersion: version,
		GrobidTs:      ts,
	}
	log.Println(doc)
	return nil
}

type GrobidDocument struct {
	GrobidVersion   string
	GrobidTs        string
	Header          GrobidBiblio
	PDFMD5          string
	LanguageCode    string
	Citations       []GrobidBiblio
	Abstract        string
	Body            string
	Acknowledgement string
	Annex           string
}

func (g *GrobidDocument) RemoveEncumbered() {
	g.Abstract = ""
	g.Body = ""
	g.Acknowledgement = ""
	g.Annex = ""
}

type GrobidAddress struct {
	AddrLine   string
	PostCode   string
	Settlement string
	Country    string
}

type GrobidAffiliation struct {
	Institution string
	Department  string
	Laboratory  string
	Address     GrobidAddress
}

type GrobidAuthor struct {
	FullName    string
	GivenName   string
	MiddleName  string
	Surname     string
	Email       string
	ORCID       string
	Affiliation GrobidAffiliation
}

type GrobidBiblio struct {
	Authors       []GrobidAuthor
	Index         int
	ID            string
	Unstructured  string
	Date          string
	Title         string
	BookTitle     string
	SeriesTitle   string
	Editor        []GrobidAuthor
	Journal       string
	JournalAbbrev string
	Publisher     string
	Institution   string
	ISSN          string
	EISSN         string
	Volume        string
	Issue         string
	Pages         string
	FirstPage     string
	LastPage      string
	Note          string
	DOI           string
	PMID          string
	PMCID         string
	ArxivID       string
	PII           string
	Ark           string
	IsTexID       string
	URL           string
}

func (g *GrobidBiblio) IsEmpty() bool {
	if len(g.Authors) > 0 || len(g.Editor) > 0 {
		return false
	}
	return !anyString(
		g.Date,
		g.Title,
		g.Journal,
		g.Publisher,
		g.Volume,
		g.Issue,
		g.Pages,
		g.DOI,
		g.PMID,
		g.PMCID,
		g.ArxivID,
		g.URL,
	)
}

// anyString returns true, if any of the given strings is not empty.
func anyString(vs ...string) bool {
	for _, v := range vs {
		if v != "" {
			return true
		}
	}
	return false
}

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

func parseAffiliation(elem *etree.Element) *GrobidAffiliation { return nil }
func parseAuthor(elem *etree.Element) *GrobidAuthor           { return nil }
func parseEditor(elem *etree.Element) *GrobidAuthor           { return nil }

func parsePersName(elem *etree.Element) *GrobidAuthor {
	if elem == nil {
		return nil
	}
	name := strings.Join(iterTextTrimSpace(elem, " "))
	ga := &GrobidAuthor{
		FullName:   name,
		GivenName:  findElementText(`./forename[@type=first]`),
		MiddleName: findElementText(`./forename[@type=middle]`),
		Surname:    findElementText(`./surname`),
	}
	return ga
}

func parseBiblio(elem *etree.Element) *GrobidBiblio { return nil }

func findElementText(elem *etree.Element, path string) string {
	e := elem.FindElement(path)
	if e == nil {
		return ""
	}
	return e.Text()
}

// iterText returns all text elements recursively, in document order.
func iterText(elem *etree.Element) (result []string) {
	result = append(result, elem.Text())
	for _, ch := range elem.ChildElements() {
		result = append(result, iterText(ch)...)
	}
	result = append(result, elem.Tail())
	return result
}

// iterTextTrimSpace returns all child text elements, recursively, in document
// order, with all whitespace stripped.
func iterTextTrimSpace(elem *etree.Element) (result []string) {
	for _, v := range iterText(elem) {
		c := strings.TrimSpace(v)
		if len(c) == 0 {
			continue
		}
		result = append(result, c)
	}
	return result
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

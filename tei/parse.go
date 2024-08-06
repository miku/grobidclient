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

func parseAffiliation(elem *etree.Element) *GrobidAffiliation {
	ga := &GrobidAffiliation{}
	for _, e := range elem.FindElements(fmt.Sprintf(`./orgName[namespace-uri=%q]`, NS)) {
		orgTypeAttr := e.SelectAttr("type")
		if orgTypeAttr == nil {
			continue
		}
		switch orgTypeAttr.Value {
		case "institution":
			ga.Institution = e.Text()
		case "department":
			ga.Department = e.Text()
		case "laboratory":
			ga.Laboratory = e.Text()
		}
	}
	if ga.isEmpty() {
		return nil
	}
	addrTag := elem.FindElement(fmt.Sprintf("./address[namespace-uri=%q]", NS))
	if addrTag != nil {
		// TODO: add address
	}
	return ga
}

// parseAuthor is an internal helper to parse a single TEI author XML tag into
// a GrobidAuthor struct. An author could appear in the document headers or
// citations.
func parseAuthor(elem *etree.Element) *GrobidAuthor {
	persNameTag := elem.FindElement(fmt.Sprintf("./persName[namespace-uri=%q]", NS))
	if persNameTag == nil {
		return nil
	}
	ga := parsePersName(elem)
	if ga == nil {
		return nil
	}
	ga.ORCID = findElementText(`./idno[@type="ORCID"]`) // TODO: NS
	ga.Email = findElementText(`./email`)               // TODO: NS
	affiliationTag := elem.FindElement(fmt.Sprintf(`./affiliation[namespace-uri=%q]`, NS))
	if affiliationTag != nil {
		ga.Affiliation = parseAffiliation(affiliationTag)
	}
	return ga
}

// parseEditor may contain multiple authors. Sometimes there is no persName,
// only a bare string under the <editor> tag. This helper should handle these
// cases.
func parseEditor(elem *etree.Element) []*GrobidAuthor {
	persNameTags := elem.FindElements(fmt.Sprintf("./persName[namespace-uri=%q]", NS))
	if len(persNameTags) == 0 {
		if elem.FindElement("*") == nil {
			rawName := elem.Text()
			trimmed := strings.TrimSpace(rawName)
			if len(rawName) > 0 && len(trimmed) > 2 {
				return []*GrobidAuthor{
					&GrobidAuthor{FullName: trimmed},
				}
			}
		}
		return nil
	}
	var persons []*GrobidAuthor
	for _, tag := range persNameTags {
		ga := parsePersName(tag)
		if ga == nil {
			continue
		}
		persons = append(persons, ga)
	}
	return persons
}

// parsePersName works on a single persName tag and returns a GrobidAuthor struct.
func parsePersName(elem *etree.Element) *GrobidAuthor {
	if elem == nil {
		return nil
	}
	name := strings.Join(iterTextTrimSpace(elem, " "))
	ga := &GrobidAuthor{
		FullName:   name,
		GivenName:  findElementText(`./forename[@type="first"]`),
		MiddleName: findElementText(`./forename[@type="middle"]`),
		Surname:    findElementText(`./surname`),
	}
	return ga
}

func parseBiblio(elem *etree.Element) *GrobidBiblio {
	var authors []*GrobidAuthor
	for _, ela := range elem.FindElements(fmt.Sprintf(`./author[namespace-uri=%q]`, NS)) {
		a := parseAuthor(ela)
		if a == nil {
			continue
		}
		authors = append(authors, a)
	}
	// TODO: editors
	var editors []*GrobidAuthor
	var editorTags = elem.FindElements(fmt.Sprintf(`.//editor[namespace-uri=%q]`, NS))
	for _, et := range editorTags {
		editors = append(editors, parseEditor(et)...)
	}
	var contribEditorTags = elem.FindElements(`.//contributor[@role="editor"]`) // TODO: NS
	for _, cet := range contribEditorTags {
		editors = append(editors, parseEditor(cet)...)
	}
	biblio := GrobidBiblio{
		Authors:      authors,
		Editors:      editors,
		ID:           elem.SelectAttrValue(`{http://www.w3.org/XML/1998/namespace}id`, ""), // TODO: check NS
		Unstructured: findElementText(elem, `.//note[@type="raw_reference"]`),              // TODO: NS
		// date below
		// titles: @level=a for article, @level=m for manuscrupt (book)
		Title:         findElementText(`.//title[@type="main"]`),
		Journal:       findElementText(`.//title[@level="j"]`),
		JournalAbbrev: findElementText(`.//title[@level="j"][@type="abbrev"]`),
		SeriesTitle:   findElementText(`.//title[@level="s"]`),
		Publisher:     findElementText(`.//publicationStmt/publisher`),
		Institution:   findElementText(`.//respStmt/orgName`),
		Volume:        findElementText(`.//biblScope[@unit="volume"]`),
		Issue:         findElementText(`.//biblScope[@unit="issue"]`),
		// pages below
		DOI:     findElementText(`.//idno[@type="DOI"]`),
		PMID:    findElementText(`.//idno[@type="PMID"]`),
		PMCID:   findElementText(`.//idno[@type="PMCID"]`),
		ArxivID: findElementText(`.//idno[@type="arXiv"]`),
		PII:     findElementText(`.//idno[@type="PII"]`),
		Ark:     findElementText(`.//idno[@type="ark"]`),
		IsTexID: findElementText(`.//idno[@type="istexId"]`),
		ISSN:    findElementText(`.//idno[@type="ISSN"]`),
		EISSN:   findElementText(`.//idno[@type="eISSN"]`),
	}
	// TODO: bookTitleTag

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

func (g *GrobidAffiliation) isEmpty() bool {
	return g.Institution == "" && g.Department == "" && g.Laboratory == "" && g.Address == nil
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

func cleanURL(u string) string {
	if len(u) == 0 {
		return u
	}
	u = strings.TrimSpace(u)
	if strings.HasSuffix(u, ".Lastaccessed") {
		u = strings.Replace(u, ".Lastaccessed", "", 1)
	}
	if strings.HasPrefix(u, "<") {
		u = u[1:]
	}
	if strings.Contains(u, ">") {
		u = strings.Split(u, ">")[0]
	}
	return u
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

// findElementText return the text of a node matched by path or the empty string.
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

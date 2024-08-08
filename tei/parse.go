// https://github.com/allenai/s2orc-doc2json/blob/71c022ed4bed3ffc71d22c2ac5cdbc133ad04e3c/doc2json/grobid2json/tei_to_json.py#L691
package tei

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/beevik/etree"
)

const (
	XMLNS = "http://www.w3.org/XML/1998/namespace"
	NS    = "http://www.tei-c.org/ns/1.0"
)

var ErrInvalidDocument = errors.New("invalid document")

// ParseCitationList parses TEI-XML of one or more references. This should work
// with either /api/processCitation or /api/processCitationList API responses
// from GROBID.
//
// Note that processed citations are usually returns as a bare XML tag, not a
// full XML document, which means that the TEI xmlns is not set. This requires
// a tweak to all downstream parsing code to handle documents with or w/o the
// namespace.
func ParseCitationList(xmlText string) []*GrobidBiblio {
	xmlText = strings.Replace(xmlText, `xmlns="http://www.tei-c.org/ns/1.0"`, ``, 1)

	tree := etree.NewDocument()
	tree.ReadFromString(xmlText)
	root := tree.Root()
	if root == nil {
		return nil
	}
	if root.Tag == "biblStruct" {
		ref := parseBiblio(root)
		ref.Index = 0
		return []*GrobidBiblio{ref}
	}
	var refs []*GrobidBiblio
	for i, bs := range tree.FindElements(`.//biblStruct`) {
		ref := parseBiblio(bs)
		ref.Index = i
		refs = append(refs, ref)
	}
	return refs
}

func ParseCitation(xmlText string) *GrobidBiblio {
	cl := ParseCitationList(xmlText)
	if len(cl) == 0 {
		return nil
	}
	c := cl[0]
	c.Index = -1
	if c.IsEmpty() {
		return nil
	}
	return c
}

func ParseCitations(xmlText string) []*GrobidBiblio {
	return ParseCitationList(xmlText)
}

func ParseDocument(r io.Reader) (*GrobidDocument, error) {
	tree := etree.NewDocument()
	_, err := tree.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	tei := tree.Root()
	if tei == nil {
		return nil, ErrInvalidDocument
	}
	header := tei.FindElement(fmt.Sprintf(".//teiHeader[namespace-uri()=%q]", NS))
	if header == nil {
		return nil, ErrInvalidDocument
	}
	applicationTags := header.FindElements(
		fmt.Sprintf(".//appInfo[namespace-uri()=%q]/application[namespace-uri()=%q]", NS, NS))
	if len(applicationTags) == 0 {
		return nil, ErrInvalidDocument
	}
	var (
		applicationTag = applicationTags[0]
		version        = strings.TrimSpace(applicationTag.SelectAttr("version").Value)
		ts             = strings.TrimSpace(applicationTag.SelectAttr("when").Value)
	)
	doc := &GrobidDocument{
		GrobidVersion: version,
		GrobidTs:      ts,
		Header:        parseBiblio(header),
		PDFMD5:        findElementText(header, `.//idno[@type="MD5"]`),
	}
	var refs []*GrobidBiblio
	for i, bs := range tei.FindElements(`.//listBibl/biblStruct`) {
		ref := parseBiblio(bs)
		ref.Index = i
		refs = append(refs, ref)
	}
	doc.Citations = refs
	textTag := tei.FindElement(`.//text`) // TODO: NS
	if textTag != nil {
		if lang := textTag.SelectAttrValue("lang", ""); lang != "" {
			// this is the 'body' language
			doc.LanguageCode = lang
		}
	}
	var el *etree.Element
	if el = tei.FindElement(`.//profileDesc/abstract`); el != nil { // TODO: NS
		doc.Abstract = strings.Join(iterTextTrimSpace(el), " ")
	}
	if el = tei.FindElement(`.//text/body`); el != nil { // TODO: NS
		doc.Body = strings.Join(iterTextTrimSpace(el), " ")
	}
	if el = tei.FindElement(`.//back/div[@type="acknowledgement"]`); el != nil {
		doc.Acknowledgement = strings.Join(iterTextTrimSpace(el), " ")
	}
	if el = tei.FindElement(`.//back/div[@type="annex"]`); el != nil {
		doc.Annex = strings.Join(iterTextTrimSpace(el), " ")
	}
	return doc, nil
}

func parseAffiliation(elem *etree.Element) *GrobidAffiliation {
	ga := &GrobidAffiliation{}
	for _, e := range elem.FindElements(`./orgName`) {
		switch e.SelectAttrValue("type", "") {
		case "institution":
			ga.Institution = e.Text()
		case "department":
			ga.Department = e.Text()
		case "laboratory":
			ga.Laboratory = e.Text()
		default:
			continue
		}
	}
	if ga.isEmpty() {
		return nil
	}
	addrTag := elem.FindElement("./address")
	if addrTag != nil {
		addr := &GrobidAddress{
			AddrLine:   findElementText(addrTag, `./addrLine`),
			PostCode:   findElementText(addrTag, `./postCode`),
			Settlement: findElementText(addrTag, `./settlement`),
			Country:    findElementText(addrTag, `./country`),
		}
		ga.Address = addr
	}
	return ga
}

// parseAuthor is an internal helper to parse a single TEI author XML tag into
// a GrobidAuthor struct. An author could appear in the document headers or
// citations.
func parseAuthor(elem *etree.Element) *GrobidAuthor {
	persNameTag := elem.FindElement("./persName")
	if persNameTag == nil {
		return nil
	}
	ga := parsePersName(persNameTag)
	if ga == nil {
		return nil
	}
	ga.ORCID = findElementText(elem, `./idno[@type="ORCID"]`) // TODO: NS
	ga.Email = findElementText(elem, `./email`)               // TODO: NS
	affiliationTag := elem.FindElement(`./affiliation`)
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
	name := strings.Join(iterTextTrimSpace(elem), " ")
	ga := &GrobidAuthor{
		FullName:   name,
		GivenName:  findElementText(elem, `./forename[@type="first"]`),
		MiddleName: findElementText(elem, `./forename[@type="middle"]`),
		Surname:    findElementText(elem, `./surname`),
	}
	return ga
}

func parseBiblio(elem *etree.Element) *GrobidBiblio {
	var authors []*GrobidAuthor
	for _, ela := range elem.FindElements(`.//author`) {
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
	biblio := &GrobidBiblio{
		Authors:      authors,
		Editors:      editors,
		ID:           elem.SelectAttrValue(`id`, ""),                          // TODO: check NS
		Unstructured: findElementText(elem, `.//note[@type="raw_reference"]`), // TODO: NS
		// date below
		// titles: @level=a for article, @level=m for manuscrupt (book)
		Title:         findElementText(elem, `.//title[@type="main"]`),
		Journal:       findElementText(elem, `.//title[@level="j"]`),
		JournalAbbrev: findElementText(elem, `.//title[@level="j"][@type="abbrev"]`),
		SeriesTitle:   findElementText(elem, `.//title[@level="s"]`),
		Publisher:     findElementText(elem, `.//publicationStmt/publisher`),
		Institution:   findElementText(elem, `.//respStmt/orgName`),
		Volume:        findElementText(elem, `.//biblScope[@unit="volume"]`),
		Issue:         findElementText(elem, `.//biblScope[@unit="issue"]`),
		// pages below
		DOI:     findElementText(elem, `.//idno[@type="DOI"]`),
		PMID:    findElementText(elem, `.//idno[@type="PMID"]`),
		PMCID:   findElementText(elem, `.//idno[@type="PMCID"]`),
		ArxivID: findElementText(elem, `.//idno[@type="arXiv"]`),
		PII:     findElementText(elem, `.//idno[@type="PII"]`),
		Ark:     findElementText(elem, `.//idno[@type="ark"]`),
		IsTexID: findElementText(elem, `.//idno[@type="istexId"]`),
		ISSN:    findElementText(elem, `.//idno[@type="ISSN"]`),
		EISSN:   findElementText(elem, `.//idno[@type="eISSN"]`),
	}
	bookTitleTag := elem.FindElement(`.//title[@level="m"]`) // TODO: NS
	if bookTitleTag != nil && bookTitleTag.SelectAttrValue("type", "") == "" {
		biblio.BookTitle = bookTitleTag.Text()
	}
	if biblio.BookTitle != "" && biblio.Title == "" {
		biblio.Title = biblio.BookTitle
		biblio.BookTitle = ""
	}
	noteTag := elem.FindElement(`.//note`)
	if noteTag != nil && noteTag.SelectAttrValue("type", "") == "" {
		biblio.Note = noteTag.Text()
	}
	if biblio.Publisher == "" {
		biblio.Publisher = findElementText(elem, `.//imprint/publisher`)
	}
	dateTag := elem.FindElement(`.//date[@type="published"]`)
	if dateTag != nil {
		biblio.Date = dateTag.SelectAttrValue("when", "")
	}
	if biblio.ArxivID != "" && strings.HasPrefix(biblio.ArxivID, "arXiv:") {
		biblio.ArxivID = biblio.ArxivID[6:]
	}
	var el *etree.Element
	el = elem.FindElement(`.//biblScope[@unit="page"]`) // TODO: NS
	if el != nil {
		if v := el.SelectAttrValue("from", ""); v != "" {
			biblio.FirstPage = v
		}
		if v := el.SelectAttrValue("to", ""); v != "" {
			biblio.LastPage = v
		}
		if biblio.FirstPage != "" && biblio.LastPage != "" {
			biblio.Pages = fmt.Sprintf("%s-%s", biblio.FirstPage, biblio.LastPage)
		} else {
			biblio.Pages = el.Text()
		}
	}
	el = elem.FindElement(`.//ptr[@target]`) // TODO: NS
	if el != nil {
		biblio.URL = cleanURL(el.SelectAttrValue("target", ""))
	}
	if biblio.DOI != "" && biblio.URL != "" {
		if strings.Contains(biblio.URL, "://doi.org/") || strings.Contains(biblio.URL, "://dx.doi.org/") {
			biblio.URL = ""
		}
	}
	return biblio
}

type GrobidDocument struct {
	GrobidVersion   string          `json:"grobid_version,omitempty"`
	GrobidTs        string          `json:"grobid_ts,omitempty"`
	Header          *GrobidBiblio   `json:"header,omitempty"`
	PDFMD5          string          `json:"pdfmd5,omitempty"`
	LanguageCode    string          `json:"lang,omitempty"`
	Citations       []*GrobidBiblio `json:"citations,omitempty"`
	Abstract        string          `json:"abstract,omitempty"`
	Body            string          `json:"body,omitempty"`
	Acknowledgement string          `json:"acknowledgement,omitempty"`
	Annex           string          `json:"annex,omitempty"`
}

func (g *GrobidDocument) RemoveEncumbered() {
	g.Abstract = ""
	g.Body = ""
	g.Acknowledgement = ""
	g.Annex = ""
}

type GrobidAddress struct {
	AddrLine   string `json:"line,omitempty"`
	PostCode   string `json:"postcode,omitempty"`
	Settlement string `json:"settlement,omitempty"`
	Country    string `json:"country,omitempty"`
}

type GrobidAffiliation struct {
	Institution string         `json:"institution,omitempty"`
	Department  string         `json:"department,omitempty"`
	Laboratory  string         `json:"laboratory,omitempty"`
	Address     *GrobidAddress `json:"address,omitempty"`
}

func (g *GrobidAffiliation) isEmpty() bool {
	return g.Institution == "" && g.Department == "" && g.Laboratory == "" && g.Address == nil
}

type GrobidAuthor struct {
	FullName    string             `json:"full_name,omitempty"`
	GivenName   string             `json:"given_name,omitempty"`
	MiddleName  string             `json:"middle_name,omitempty"`
	Surname     string             `json:"surname,omitempty"`
	Email       string             `json:"email,omitempty"`
	ORCID       string             `json:"orcid,omitempty"`
	Affiliation *GrobidAffiliation `json:"aff,omitempty"`
}

type GrobidBiblio struct {
	Authors       []*GrobidAuthor `json:"authors,omitempty"`
	Index         int             `json:"index,omitempty"`
	ID            string          `json:"id,omitempty"`
	Unstructured  string          `json:"unstructured,omitempty"`
	Date          string          `json:"date,omitempty"`
	Title         string          `json:"title,omitempty"`
	BookTitle     string          `json:"book_title,omitempty"`
	SeriesTitle   string          `json:"series_title,omitempty"`
	Editors       []*GrobidAuthor `json:"editors,omitempty"`
	Journal       string          `json:"journal,omitempty"`
	JournalAbbrev string          `json:"journal_abbrev,omitempty"`
	Publisher     string          `json:"publisher,omitempty"`
	Institution   string          `json:"institution,omitempty"`
	ISSN          string          `json:"issn,omitempty"`
	EISSN         string          `json:"eissn,omitempty"`
	Volume        string          `json:"volume,omitempty"`
	Issue         string          `json:"issue,omitempty"`
	Pages         string          `json:"pages,omitempty"`
	FirstPage     string          `json:"first_page,omitempty"`
	LastPage      string          `json:"last_page,omitempty"`
	Note          string          `json:"note,omitempty"`
	DOI           string          `json:"doi,omitempty"`
	PMID          string          `json:"pmid,omitempty"`
	PMCID         string          `json:"pmcid,omitempty"`
	ArxivID       string          `json:"arxiv_id,omitempty"`
	PII           string          `json:"pii,omitempty"`
	Ark           string          `json:"ark,omitempty"`
	IsTexID       string          `json:"is_tex_id,omitempty"`
	URL           string          `json:"url,omitempty"`
}

func (g *GrobidBiblio) IsEmpty() bool {
	if len(g.Authors) > 0 || len(g.Editors) > 0 {
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
	if elem == nil {
		return ""
	}
	e := elem.FindElement(path)
	if e == nil {
		return ""
	}
	return e.Text()
}

// iterText returns all text elements recursively, in document order.
func iterText(elem *etree.Element) (result []string) {
	if elem == nil {
		return
	}
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
	if elem == nil {
		return
	}
	for _, v := range iterText(elem) {
		c := strings.TrimSpace(v)
		if len(c) == 0 {
			continue
		}
		result = append(result, c)
	}
	return result
}

package tei

import (
	"encoding/json"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/andreyvit/diff"
	"github.com/beevik/etree"
)

func TestParseBiblio(t *testing.T) {}

func TestExampleGrobidTei(t *testing.T) {
	f, err := os.Open("../testdata/document/example.tei.xml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	defer f.Close()
	doc, err := ParseDocument(f)
	if err != nil {
		t.Fatalf("got %v, want %v", err, nil)
	}
	var want string
	want = `Changes of patients' satisfaction with the health care services in Lithuanian Health Promoting Hospitals network`
	if doc.Header.Title != want {
		t.Fatalf("title: got %v, want %v", doc.Header.Title, want)
	}
	var ref *GrobidBiblio
	for _, c := range doc.Citations {
		if c.ID == "b12" {
			ref = c
			break
		}
	}
	if ref == nil {
		t.Fatalf("expected a non-nil ref")
	}
	if len(ref.Authors) == 0 {
		t.Fatalf("expected authors")
	}
	author0 := ref.Authors[0]
	if want := "K Tasa"; author0.FullName != want {
		t.Fatalf("got %v, want %v", author0.FullName, want)
	}
	if want := "K"; author0.GivenName != want {
		t.Fatalf("got %v, want %v", author0.GivenName, want)
	}
	if want := "Tasa"; author0.Surname != want {
		t.Fatalf("got %v, want %v", author0.Surname, want)
	}
	if want := "Quality Management in Health Care"; ref.Journal != want {
		t.Fatalf("got %v, want %v", ref.Journal, want)
	}
	if want := "Using patient feedback for quality improvement"; ref.Title != want {
		t.Fatalf("got %v, want %v", ref.Title, want)
	}
	if want := "1996"; ref.Date != want {
		t.Fatalf("got %v, want %v", ref.Date, want)
	}
	if want := "206-225"; ref.Pages != want {
		t.Fatalf("got %v, want %v", ref.Pages, want)
	}
	if want := "8"; ref.Volume != want {
		t.Fatalf("got %v, want %v", ref.Volume, want)
	}
	want = `Tasa K, Baker R, Murray M. Using patient feedback for qua- lity improvement. Quality Management in Health Care 1996;8:206-19.`
	if ref.Unstructured != want {
		t.Fatalf("got %v, want %v", ref.Unstructured, want)
	}

}

func TestInvalidXML(t *testing.T) {
	var err error
	_, err = ParseDocument(strings.NewReader(`this is not XML`))
	if err != ErrInvalidDocument {
		t.Fatalf("got %v, want %v", err, ErrInvalidDocument)
	}
	_, err = ParseDocument(strings.NewReader(`<xml></xml>`))
	if err != ErrInvalidDocument {
		t.Fatalf("got %v, want %v", err, ErrInvalidDocument)
	}
	doc := ParseCitations(`this is not XML`)
	if doc != nil {
		t.Fatalf("got %v, want %v", doc, nil)
	}
}

// TestParseSmall tests parsing. Use TEST_SNAPSHOT=1 for creating a snapshot.
func TestParseSmall(t *testing.T) {
	f, err := os.Open("../testdata/small.xml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	defer f.Close()
	doc, err := ParseDocument(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	got := strings.TrimSpace(string(b)) // JSON view of parsed data
	snapshotFn := "../testdata/small.json"
	switch os.Getenv("TEST_SNAPSHOT") {
	case "1", "true", "yes", "on":
		t.Logf("writing snapshot to %s", snapshotFn)
		if err := os.WriteFile(snapshotFn, b, 0755); err != nil {
			t.Fatalf("failed to write snapshot: %v", err)
		}
	default:
		b, err = os.ReadFile(snapshotFn) // snapshot
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		want := strings.TrimSpace(string(b))
		if got != want {
			t.Fatalf("parse mismatch: %s", diff.LineDiff(got, want))
		}
	}
}

func TestCleanURL(t *testing.T) {
	var cases = []struct {
		about  string
		u      string
		result string
	}{
		{
			about:  "empty",
			u:      "",
			result: "",
		},
		{
			about:  "already ok",
			u:      "http://archive.org",
			result: "http://archive.org",
		},
		{
			about:  "last accessed",
			u:      "http://archive.org.Lastaccessed",
			result: "http://archive.org",
		},
		{
			about:  "< prefix",
			u:      "<http://archive.org.Lastaccessed",
			result: "http://archive.org",
		},
		// TODO: add more
	}
	for _, c := range cases {
		result := cleanURL(c.u)
		if result != c.result {
			t.Fatalf("[%s] got %v, want %v", c.about, result, c.result)
		}
	}
}

func TestAnyString(t *testing.T) {
	var cases = []struct {
		about  string
		vs     []string
		result bool
	}{
		{
			about:  "nil",
			vs:     nil,
			result: false,
		},
		{
			about:  "no values",
			vs:     []string{},
			result: false,
		},
		{
			about:  "3 empty strings",
			vs:     []string{"", "", ""},
			result: false,
		},
		{
			about:  "3 empty strings, 1 non-empty",
			vs:     []string{"", "", "", "x"},
			result: true,
		},
		{
			about:  "3 empty strings, 2 non-empty",
			vs:     []string{"", "", "", "x", "y"},
			result: true,
		},
	}
	for _, c := range cases {
		result := anyString(c.vs...)
		if result != c.result {
			t.Fatalf("[%s] got %v, want %v", c.about, result, c.result)
		}
	}
}

func TestFindElementText(t *testing.T) {
	var cases = []struct {
		about  string
		elem   *etree.Element
		path   string
		result string
	}{
		{
			about:  "nil element",
			elem:   mustElementFromString(""),
			path:   "",
			result: "",
		},
	}
	for _, c := range cases {
		result := findElementText(c.elem, c.path)
		if !reflect.DeepEqual(result, c.result) {
			t.Fatalf("[%s] got %v, want %v", c.about, result, c.result)
		}
	}
}

func TestIterTextTrimSpace(t *testing.T) {
	var cases = []struct {
		about  string
		input  *etree.Element
		result []string
	}{
		{
			"empty string",
			mustElementFromString(""),
			nil,
		},
		{
			"1 tag, no text",
			mustElementFromString("<a></a>"),
			nil,
		},
		{
			"1 tag, text",
			mustElementFromString("<a>hello</a>"),
			[]string{"hello"},
		},
		{
			"2 tags, text",
			mustElementFromString("<a>hello <b>world</b></a>"),
			[]string{"hello", "world"},
		},
		{
			"2 tags, text, tail",
			mustElementFromString("<a>hello <b>world</b>!</a>"),
			[]string{"hello", "world", "!"},
		},
		{
			"3 tags, text, tail, whitespace",
			mustElementFromString("<a>hello <b>world</b><b>...  </b>  !</a>"),
			[]string{"hello", "world", "...", "!"},
		},
	}
	for _, c := range cases {
		result := iterTextTrimSpace(c.input)
		if !reflect.DeepEqual(result, c.result) {
			t.Fatalf("[%s] got %v, want %v", c.about, result, c.result)
		}
	}
}

func TestSingleCitations(t *testing.T) {
	var data = `
<biblStruct>
    <analytic>
        <title level="a" type="main">Mesh migration following abdominal hernia repair: a comprehensive review</title>
        <author>
            <persName
                xmlns="http://www.tei-c.org/ns/1.0">
                <forename type="first">H</forename>
                <forename type="middle">B</forename>
                <surname>Cunningham</surname>
            </persName>
        </author>
        <author>
            <persName
                xmlns="http://www.tei-c.org/ns/1.0">
                <forename type="first">J</forename>
                <forename type="middle">J</forename>
                <surname>Weis</surname>
            </persName>
        </author>
        <author>
            <persName
                xmlns="http://www.tei-c.org/ns/1.0">
                <forename type="first">L</forename>
                <forename type="middle">R</forename>
                <surname>Taveras</surname>
            </persName>
        </author>
        <author>
            <persName
                xmlns="http://www.tei-c.org/ns/1.0">
                <forename type="first">S</forename>
                <surname>Huerta</surname>
            </persName>
        </author>
        <idno type="DOI">10.1007/s10029-019-01898-9</idno>
        <idno type="PMID">30701369</idno>
    </analytic>
    <monogr>
        <title level="j">Hernia</title>
        <imprint>
            <biblScope unit="volume">23</biblScope>
            <biblScope unit="issue">2</biblScope>
            <biblScope unit="page" from="235" to="243" />
            <date type="published" when="2019-01-30" />
        </imprint>
    </monogr>
</biblStruct>"
	`
	doc := ParseCitation(data)
	if doc == nil {
		t.Fatal("expected non nil result")
	}
	if doc.IsEmpty() != false {
		t.Fatal("empty: want false")
	}
	if want := `Mesh migration following abdominal hernia repair: a comprehensive review`; doc.Title != want {
		t.Fatalf("got %s, want %v", doc.Title, want)
	}
	if len(doc.Authors) < 3 {
		t.Fatalf("expeted at least 3 authors")
	}
	if want := `L`; doc.Authors[2].GivenName != want {
		t.Fatalf("got %v, want %v", doc.Authors[2].GivenName, want)
	}
	if want := `R`; doc.Authors[2].MiddleName != want {
		t.Fatalf("got %v, want %v", doc.Authors[2].MiddleName, want)
	}
	if want := `Taveras`; doc.Authors[2].Surname != want {
		t.Fatalf("got %v, want %v", doc.Authors[2].Surname, want)
	}
	if want := `L R Taveras`; doc.Authors[2].FullName != want {
		t.Fatalf("got %v, want %v", doc.Authors[2].FullName, want)
	}
	if want := `10.1007/s10029-019-01898-9`; doc.DOI != want {
		t.Fatalf("got %v, want %v", doc.DOI, want)
	}
	if want := `30701369`; doc.PMID != want {
		t.Fatalf("got %v, want %v", doc.PMID, want)
	}
	if want := `2019-01-30`; doc.Date != want {
		t.Fatalf("got %v, want %v", doc.Date, want)
	}
	if want := `235-243`; doc.Pages != want {
		t.Fatalf("got %v, want %v", doc.Pages, want)
	}
	if want := `235`; doc.FirstPage != want {
		t.Fatalf("got %v, want %v", doc.FirstPage, want)
	}
	if want := `243`; doc.LastPage != want {
		t.Fatalf("got %v, want %v", doc.LastPage, want)
	}
	if want := `2`; doc.Issue != want {
		t.Fatalf("got %v, want %v", doc.Issue, want)
	}
	if want := `Hernia`; doc.Journal != want {
		t.Fatalf("got %v, want %v", doc.Journal, want)
	}
}

// mustElementFromString returns the root element from a given XML snippet. Will
// panic, if the XML is not parseable.
func mustElementFromString(xmlText string) *etree.Element {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlText); err != nil {
		panic(err)
	}
	return doc.Root()
}

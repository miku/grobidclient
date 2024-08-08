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

// mustElementFromString returns the root element from a given XML snippet. Will
// panic, if the XML is not parseable.
func mustElementFromString(xmlText string) *etree.Element {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlText); err != nil {
		panic(err)
	}
	return doc.Root()

}

// // diffJsonFile compares a JSON serialization of v with the content of a file.
// func diffJsonFile(v any, filename expected) ([]diffpathmatch.Diff, error) {
// 	var buf bytes.Buffer
// 	enc := json.NewEncoder(&buf)
// 	if err := enc.Encode(v); err != nil {
// 		return nil, err
// 	}
// 	b, err := os.ReadFile(filename)
// 	if err != nil {
// 		return nil, err
// 	}
// 	dmp := diffmatchpatch.New()
// 	diffs := dmp.DiffMain(buf.String(), string(b), false)
// 	return diffs, nil
//
// }

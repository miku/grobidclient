package tei

import (
	"reflect"
	"testing"

	"github.com/beevik/etree"
)

func TestIterTextTrimSpace(t *testing.T) {
	var cases = []struct {
		about  string
		input  *etree.Element
		result []string
	}{
		{
			"empty string",
			elementFromString(""),
			nil,
		},
		{
			"1 tag, no text",
			elementFromString("<a></a>"),
			nil,
		},
		{
			"1 tag, text",
			elementFromString("<a>hello</a>"),
			[]string{"hello"},
		},
		{
			"2 tags, text",
			elementFromString("<a>hello <b>world</b></a>"),
			[]string{"hello", "world"},
		},
		{
			"2 tags, text, tail",
			elementFromString("<a>hello <b>world</b>!</a>"),
			[]string{"hello", "world", "!"},
		},
		{
			"3 tags, text, tail, whitespace",
			elementFromString("<a>hello <b>world</b><b>...  </b>  !</a>"),
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

// elementFromString returns the root element from a given XML snippet. Will
// panic, if the XML is not parseable.
func elementFromString(xmlText string) *etree.Element {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlText); err != nil {
		panic(err)
	}
	return doc.Root()

}

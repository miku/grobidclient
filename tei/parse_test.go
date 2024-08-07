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

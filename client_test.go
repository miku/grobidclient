package grobidclient

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseLines(t *testing.T) {
	var cases = []struct {
		about  string
		r      io.Reader
		result []string
		err    error
	}{
		{
			about:  "nothing to read",
			r:      strings.NewReader(``),
			result: nil,
			err:    nil,
		},
		{
			about:  "single line",
			r:      strings.NewReader("1\n"),
			result: []string{"1"},
			err:    nil,
		},
		{
			about:  "just an empty line",
			r:      strings.NewReader("\n"),
			result: nil,
			err:    nil,
		},
		{
			about:  "just an empty line",
			r:      strings.NewReader("1\n2\n3  \n"),
			result: []string{"1", "2", "3"},
			err:    nil,
		},
	}
	for _, c := range cases {
		var lines []string
		err := parseLines(c.r, &lines)
		if err != c.err {
			t.Fatalf("[%s] got %v, want %v", c.about, err, c.err)
		}
		if !reflect.DeepEqual(lines, c.result) {
			t.Fatalf("[%s] got %v (%d), want %v (%d)", c.about, lines, len(lines), c.result, len(c.result))
		}
	}
}

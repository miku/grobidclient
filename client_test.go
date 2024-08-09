package grobidclient

import (
	"io"
	"os"
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
		lines, err := parseLines(c.r)
		if err != c.err {
			t.Fatalf("[%s] got %v, want %v", c.about, err, c.err)
		}
		if !reflect.DeepEqual(lines, c.result) {
			t.Fatalf("[%s] got %v (%d), want %v (%d)", c.about, lines, len(lines), c.result, len(c.result))
		}
	}
}

func TestDefaultResultWriter(t *testing.T) {
	var cases = []struct {
		about  string
		result *Result
		opts   *Options
		dst    string // destination file
		err    error
	}{
		{
			about:  "nil",
			result: nil,
			opts:   nil,
			dst:    "",
			err:    nil,
		},
		{
			about:  "empty result",
			result: &Result{},
			opts:   nil,
			dst:    "",
			err:    nil,
		},
		{
			about: "only 200",
			result: &Result{
				StatusCode: 200,
			},
			opts: nil,
			dst:  "_200.txt",
			err:  nil,
		},
		{
			about: "only 200, zero body",
			result: &Result{
				Filename:   "zerobody.jpg",
				StatusCode: 200,
			},
			opts: nil,
			dst:  "zerobody_200.txt",
			err:  nil,
		},
		{
			about: "only 200, 1 byte body",
			result: &Result{
				Filename:   "1byte.txt",
				StatusCode: 200,
				Body:       []byte{'1'},
			},
			opts: nil,
			dst:  "1byte.grobid.tei.xml",
			err:  nil,
		},
	}
	for _, c := range cases {
		err := DefaultResultWriter(c.result, c.opts)
		if err != c.err {
			t.Fatalf("got %v, want %v", err, c.err)
		}
		if c.dst != "" {
			if _, err := os.Stat(c.dst); os.IsNotExist(err) {
				t.Errorf("expected file %v as side effect", c.dst)
			}
			// TODO: rework result writer interface, so it a bit less awkward to test
			if _, err := os.Stat(c.dst); err == nil {
				t.Logf("cleanup: %v", c.dst)
				os.Remove(c.dst)
			}
		}
	}
}

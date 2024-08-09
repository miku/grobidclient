package grobidclient

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"reflect"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestProcessPDF(t *testing.T) {
	skipNoDocker(t)
	if testing.Short() {
		t.Skip("skipping testcontainer based tests in short mode")
	}
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "grobid/grobid:0.8.0",
		ExposedPorts: []string{"8070/tcp"},
		WaitingFor:   wait.ForListeningPort("8070/tcp"),
	}
	grobidC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Could not start grobid: %s", err)
	}
	defer func() {
		if err := grobidC.Terminate(ctx); err != nil {
			t.Fatalf("Could not stop grobid: %s", err)
		}
	}()
	ip, err := grobidC.Host(ctx)
	if err != nil {
		t.Fatalf("TC: count not get host: %v", err)
	}
	port, err := grobidC.MappedPort(ctx, "8070")
	if err != nil {
		t.Fatalf("TC: count not get port: %v", err)
	}
	hostPort := fmt.Sprintf("http://%s:%s", ip, port.Port())
	t.Logf("starting e2e test, using grobid container running at %v", hostPort)
	grobid := New(hostPort)
	result, err := grobid.ProcessPDF(
		"testdata/pdf/062RoisinAronAmericanNaturalist03.pdf",
		"processFulltextDocument",
		nil)
	if err != nil {
		t.Fatalf("expected successful parse, got %v", err)
	}
	if result.StatusCode != 200 {
		t.Fatalf("expected HTTP 200, got %v: %v", result.StatusCode, string(result.Body))
	}
}

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

func skipNoDocker(t *testing.T) {
	noDocker := false
	cmd := exec.Command("systemctl", "is-active", "docker")
	b, err := cmd.CombinedOutput()
	if err != nil {
		noDocker = true
	}
	if strings.TrimSpace(string(b)) != "active" {
		noDocker = true
	}
	if !noDocker {
		// We found some docker.
		return
	}
	// Otherwise, try podman.
	_, err = exec.LookPath("podman")
	if err == nil {
		t.Logf("podman detected")
		// DOCKER_HOST=unix:///run/user/$UID/podman/podman.sock
		usr, err := user.Current()
		if err != nil {
			t.Logf("cannot get UID, set DOCKER_HOST manually")
		} else {
			sckt := fmt.Sprintf("unix:///run/user/%v/podman/podman.sock", usr.Uid)
			os.Setenv("DOCKER_HOST", sckt)
			t.Logf("set DOCKER_HOST to %v", sckt)
		}
		noDocker = false
	}
	if noDocker {
		t.Skipf("docker not installed or not running")
	}
}

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/miku/grobidclient"
)

var (
	server      = flag.String("S", "http://localhost:8070", "server URL")
	serviceName = flag.String("s", "processFulltextDocument", "a valid service name")
	inputDir    = flag.String("d", ".", "input directory to look for PDF, txt, or XML files")
	outputDir   = flag.String("O", "", "output directory to write parsed files to")
	configFile  = flag.String("c", "config.json", "path to config file")
	numWorkers  = flag.Int("n", runtime.NumCPU(), "number of concurrent requests")
	// flags
	generateIDs            = flag.Bool("gi", false, "generate ids")
	consolidateCitations   = flag.Bool("cc", false, "consolidate citations")
	consolidateHeader      = flag.Bool("ch", false, "consolidate header")
	includeRawCitations    = flag.Bool("irc", false, "include raw citations")
	includeRawAffiliations = flag.Bool("ira", false, "include raw affiliations")
	forceReprocess         = flag.Bool("f", false, "force reprocess")
	teiCoordinates         = flag.Bool("tei", false, "add pdf coordinates")
	segmentSentences       = flag.Bool("ss", false, "segment sentences")
	verbose                = flag.Bool("v", false, "be verbose")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `
░░      ░░░       ░░░░      ░░░       ░░░        ░░░      ░░░  ░░░░░░░░        ░
▒  ▒▒▒▒▒▒▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒▒▒▒▒▒▒▒  ▒▒▒▒
▓  ▓▓▓   ▓▓       ▓▓▓  ▓▓▓▓  ▓▓       ▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓
█  ████  ██  ███  ███  ████  ██  ████  █████  █████  ████  ██  ███████████  ████
██      ███  ████  ███      ███       ███        ███      ███        ██        █
                                                                                `)
		fmt.Fprintln(os.Stderr, "valid service names:\n")
		for _, s := range grobidclient.ValidServices {
			fmt.Fprintf(os.Stderr, "  %s\n", s)
		}
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
	}
	flag.Parse()
	if !grobidclient.IsValidService(*serviceName) {
		log.Fatal("invalid service name")
	}
	grobid := grobidclient.Grobid{
		Server: *server,
		Client: http.DefaultClient,
	}
	log.Println(grobid)
}

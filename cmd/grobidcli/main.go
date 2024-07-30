package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/miku/grobidclient"
)

var (
	server      = flag.String("S", "http://localhost:8070", "server URL")
	serviceName = flag.String("s", "processFulltextDocument", "a valid service name")
	inputDir    = flag.String("d", ".", "input directory to look for PDF, txt, or XML files")
	outputDir   = flag.String("O", "", "output directory to write parsed files to")
	configFile  = flag.String("c", "config.json", "path to config file")
	numWorkers  = flag.Int("n", runtime.NumCPU(), "number of concurrent requests")
	doPing      = flag.Bool("P", false, "do a ping")
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

type Config struct {
	BatchSize    int64    `json:"batch_size"`
	Coordinates  []string `json:"coordinates"`
	GrobidServer string   `json:"grobid_server"`
	SleepTime    int64    `json:"sleep_time"`
	Timeout      int64    `json:"timeout"`
}

func (c *Config) FromFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, c)
}

var DefaultConfig = &Config{
	BatchSize:    100,
	Coordinates:  []string{"persName", "figure", "ref", "biblStruct", "formula", "s", "note", "title"},
	Timeout:      60,
	SleepTime:    5,
	GrobidServer: "http://localhost:8070",
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `
░░      ░░░       ░░░░      ░░░       ░░░        ░░       ░░░░      ░░░  ░░░░░░░░        ░
▒  ▒▒▒▒▒▒▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒▒▒▒▒▒▒▒  ▒▒▒▒
▓  ▓▓▓   ▓▓       ▓▓▓  ▓▓▓▓  ▓▓       ▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓
█  ████  ██  ███  ███  ████  ██  ████  █████  █████  ████  ██  ████  ██  ███████████  ████
██      ███  ████  ███      ███       ███        ██       ████      ███        ██        █
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
	config := DefaultConfig
	if *configFile != "" {
		if err := config.FromFile(*configFile); err != nil {
			log.Fatal(err)
		}
	}
	grobid := grobidclient.Grobid{
		Server: *server,
		Client: http.DefaultClient, // TODO: timeouts
	}
	if *doPing {
		fmt.Printf("grobid service at %s status: %s -- %s\n",
			*server, grobid.Pingmoji(), time.Now().Format(time.RFC1123))
		os.Exit(0)
	}
	opts := &grobidclient.Options{
		GenerateIDs:            *generateIDs,
		ConsolidateHeader:      *consolidateHeader,
		ConsolidateCitations:   *consolidateCitations,
		IncludeRawCitations:    *includeRawCitations,
		IncluseRawAffiliations: *includeRawAffiliations,
		TEICoordinates:         *teiCoordinates,
		SegmentSentences:       *segmentSentences,
		Force:                  *forceReprocess,
		Verbose:                *verbose,
	}
	if err := grobid.Ping(); err != nil {
		log.Fatal(err)
	}
	result, err := grobid.ProcessPDF("fixtures/062RoisinAronAmericanNaturalist03.pdf", *serviceName, opts)
	if err != nil {
		log.Fatal(err)
	}
	if result.StatusCode == 200 {
		fmt.Println(result)
	} else {
		log.Fatal(result)
	}
}

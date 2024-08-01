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
	"github.com/sethgrid/pester"
)

var (
	server      = flag.String("S", "http://localhost:8070", "server URL") // TODO: make this repeatable
	serviceName = flag.String("s", "processFulltextDocument", "a valid service name")
	inputFile   = flag.String("f", "", "single input file to process")
	inputDir    = flag.String("d", "", "input directory to scan for PDF, txt, or XML files")
	outputDir   = flag.String("O", "", "output directory to write parsed files to")
	configFile  = flag.String("c", "config.json", "path to config file")
	numWorkers  = flag.Int("n", runtime.NumCPU()/2, "number of concurrent workers")
	doPing      = flag.Bool("P", false, "do a ping")
	// flags
	generateIDs            = flag.Bool("gi", false, "generate ids")
	consolidateCitations   = flag.Bool("cc", false, "consolidate citations")
	consolidateHeader      = flag.Bool("ch", false, "consolidate header")
	includeRawCitations    = flag.Bool("irc", false, "include raw citations")
	includeRawAffiliations = flag.Bool("ira", false, "include raw affiliations")
	forceReprocess         = flag.Bool("force", false, "force reprocess")
	teiCoordinates         = flag.Bool("tei", false, "add pdf coordinates")
	segmentSentences       = flag.Bool("ss", false, "segment sentences")
	verbose                = flag.Bool("v", false, "be verbose")
	maxRetries             = flag.Int("r", 10, "max retries")
	timeout                = flag.Duration("T", 60*time.Second, "client timeout")
	showVersion            = flag.Bool("version", false, "show version")
)

// Config is taken from the Python client implementation, which differs a bit.
// We do not need sleep time (handled by exponential backoff), and batch size.
//
// If a config file is present, server, timeout and coordinates are taken from
// the file.
type Config struct {
	BatchSize    int64    `json:"batch_size"`
	Coordinates  []string `json:"coordinates"`
	GrobidServer string   `json:"grobid_server"`
	SleepTime    int64    `json:"sleep_time"`
	Timeout      int64    `json:"timeout"`
}

func (c *Config) TimeoutDuration() time.Duration {
	dur, err := time.ParseDuration(fmt.Sprintf("%ds", c.Timeout))
	if err != nil {
		panic(err)
	}
	return dur
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
	BatchSize:    100, // unused
	Coordinates:  []string{"persName", "figure", "ref", "biblStruct", "formula", "s", "note", "title"},
	Timeout:      60,
	SleepTime:    5, // unused
	GrobidServer: *server,
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
	if *showVersion {
		fmt.Println(grobidclient.Version)
		os.Exit(1)
	}
	if !grobidclient.IsValidService(*serviceName) {
		log.Fatal("invalid service name")
	}
	config := DefaultConfig
	if *configFile != "" {
		if err := config.FromFile(*configFile); err != nil {
			log.Fatal(err)
		}
		*server = config.GrobidServer
		*timeout = config.TimeoutDuration()
	}
	hc := &http.Client{
		Timeout: *timeout,
	}
	client := pester.NewExtendedClient(hc)
	switch {
	case *doPing:
		// Ping should come back fast.
		hc.Timeout = 5 * time.Second
		client.MaxRetries = 1
		client.Backoff = pester.ExponentialBackoff
		client.RetryOnHTTP429 = false
	default:
		client.MaxRetries = *maxRetries
		client.Backoff = pester.ExponentialBackoff
		client.RetryOnHTTP429 = true
	}
	grobid := grobidclient.Grobid{
		Server: *server,
		Client: client,
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
		OutputDir:              *outputDir,
	}
	if err := grobid.Ping(); err != nil {
		log.Fatal(err)
	}
	switch {
	case *inputFile != "":
		result, err := grobid.ProcessPDF(*inputFile, *serviceName, opts)
		if err != nil {
			log.Fatal(err)
		}
		if result.StatusCode == 200 {
			log.Printf("file: %s", result.Filename)
			fmt.Println(result.StringBody())
		} else {
			log.Fatal(result)
		}
	case *inputDir != "":
		log.Printf("scanning %s", *inputDir)
		err := grobid.ProcessDirRecursive(*inputDir, *serviceName,
			*numWorkers, grobidclient.DebugResultWriter, opts)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Println("file (-f) or directory (-d) required")
	}
}

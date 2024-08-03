package main

import (
	"bufio"
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
	"github.com/slyrz/warc"
)

var (
	server             = flag.String("S", "http://localhost:8070", "server URL") // TODO: make this repeatable
	serviceName        = flag.String("s", "processFulltextDocument", "a valid service name")
	inputFile          = flag.String("f", "", "single input file to process")
	inputDir           = flag.String("d", "", "input directory to scan for PDF, txt, or XML files")
	outputDir          = flag.String("O", "", "output directory to write parsed files to")
	createHashSymlinks = flag.Bool("H", false, "use sha1 of file contents as the filename")
	configFile         = flag.String("c", "", "path to config file, often config.json")
	numWorkers         = flag.Int("n", recommendedNumWorkers(), "number of concurrent workers")
	doPing             = flag.Bool("P", false, "do a ping, then exit")
	debug              = flag.Bool("debug", false, "use debug result writer, does not create any files")
	warcFile           = flag.String("W", "", "path to WARC file to extract PDFs and parse them (experimental)")
	verbose            = flag.Bool("v", false, "be verbose")
	maxRetries         = flag.Int("r", 10, "max retries")
	timeout            = flag.Duration("T", 60*time.Second, "client timeout")
	showVersion        = flag.Bool("version", false, "show version")
	// Flags passed to grobid API.
	generateIDs            = flag.Bool("gi", false, "generate ids")
	consolidateCitations   = flag.Bool("cc", false, "consolidate citations")
	consolidateHeader      = flag.Bool("ch", false, "consolidate header")
	includeRawCitations    = flag.Bool("irc", false, "include raw citations")
	includeRawAffiliations = flag.Bool("ira", false, "include raw affiliations")
	forceReprocess         = flag.Bool("force", false, "force reprocess")
	teiCoordinates         = flag.Bool("tei", false, "add pdf coordinates")
	segmentSentences       = flag.Bool("ss", false, "segment sentences")
)

func recommendedNumWorkers() int {
	// keep the concurrency at the client (number of simultaneous calls)
	// slightly higher than the available number of threads at the server side,
	// for instance if the server has 16 threads, use a concurrency between 20
	// and 24 (it's the option n in the above mentioned clients, in my case I
	// used 24) -- https://github.com/kermitt2/grobid/issues/443#issuecomment-505208132
	ncpu := runtime.NumCPU()
	return int(float64(ncpu) * 1.5)
}

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

// Timeout returns the timeout as a time.Duration.
func (c *Config) TimeoutDuration() time.Duration {
	dur, err := time.ParseDuration(fmt.Sprintf("%ds", c.Timeout))
	if err != nil {
		panic(err)
	}
	return dur
}

// FromFile reads config from a given filename.
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

// DefaultConfig is taken from the example in the Python client. Some fields
// are not used in this client.
var DefaultConfig = &Config{
	Coordinates:  []string{"persName", "figure", "ref", "biblStruct", "formula", "s", "note", "title"},
	Timeout:      60,
	GrobidServer: *server,
	BatchSize:    100, // unused, we use worker threads
	SleepTime:    5,   // unused, covered by retry and backoff
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "grobidcli | valid service (-s) names:\n")
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
		// TODO: pester will retry on all 5XX errors, not just 503, like the
		// python client
		client.MaxRetries = *maxRetries
		client.Backoff = pester.ExponentialBackoff
		client.RetryOnHTTP429 = true
	}
	grobid := grobidclient.Grobid{
		Server: *server,
		Client: client,
	}
	if *doPing {
		fmt.Printf(`{"server": %q, "status": %q, "t": %q}`,
			*server, grobid.Pingmoji(), time.Now().Format(time.RFC1123))
		fmt.Println()
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
		CreateHashSymlinks:     *createHashSymlinks,
	}
	switch {
	case *inputFile != "":
		result, err := grobid.ProcessPDF(*inputFile, *serviceName, opts)
		if err != nil {
			log.Fatal(err)
		}
		if result.StatusCode == 200 {
			fmt.Println(result.StringBody())
		} else {
			log.Fatal(result)
		}
	case *inputDir != "":
		log.Printf("scanning %s...", *inputDir)
		var rwf grobidclient.ResultWriterFunc
		switch {
		case *debug:
			rwf = grobidclient.DebugResultWriter
		default:
			rwf = grobidclient.DefaultResultWriter
		}
		err := grobid.ProcessDirRecursive(*inputDir, *serviceName,
			*numWorkers, rwf, opts)
		if err != nil {
			log.Fatal(err)
		}
	case *warcFile != "":
		// WIP: first run with vanilla docker image
		//
		// 2024/08/02 23:06:00 processed 1098 docs, with 0 errors
		//
		// real    14m50.127s
		// user    0m21.236s
		// sys     0m6.631s
		//
		// That's 1.25 PDF/s - probably vanilla grobid could be improved.
		log.Println("scanning WARC...")
		f, err := os.Open(*warcFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		reader, err := warc.NewReader(f)
		if err != nil {
			log.Fatal(err)
		}
		// Extract all HTTP 200 PDF files into this directory.
		dir, err := os.MkdirTemp("", "grobidcli-warc-batch-*")
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			_ = os.RemoveAll(dir)
		}()
		for {
			// experimental: WARC PDF to structured metadata
			record, err := reader.ReadRecord()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			if record.Header.Get("warc-type") != "response" {
				continue
			}
			br := bufio.NewReader(record.Content)
			resp, err := http.ReadResponse(br, nil)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()
			uri := record.Header.Get("warc-target-uri")
			switch {
			case resp.StatusCode == 200:
				if err != nil {
					log.Fatal(err)
				}
				f, err := os.CreateTemp(dir, "grobidcli-extracted-*")
				if err != nil {
					log.Fatal(err)
				}
				n, err := io.Copy(f, resp.Body)
				if err != nil {
					log.Printf("copy: %v (n=%d)", err, n)
					continue
				}
				if err := f.Close(); err != nil {
					log.Fatal(err)
				}
				log.Printf("%d %s %s", resp.StatusCode, uri, f.Name())
			case resp.StatusCode >= 300 && resp.StatusCode < 400:
				location, err := resp.Location()
				if err != nil {
					log.Println(err)
				} else {
					log.Printf("%d %s => %s", resp.StatusCode, uri, location)
				}
			}
		}
		if err := grobid.ProcessDirRecursive(dir, "processFulltextDocument", 24, grobidclient.DebugResultWriter, opts); err != nil {
			log.Fatal(err)
		}
	default:
		log.Println("file (-f) or directory (-d) required, use (-P) for ping")
	}
}

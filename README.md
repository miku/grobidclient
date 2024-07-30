# grobidclient

Go client for [grobid](https://github.com/kermitt2/grobid).


## Usage

```
./grobidcli -h

░░      ░░░       ░░░░      ░░░       ░░░        ░░       ░░░░      ░░░  ░░░░░░░░        ░
▒  ▒▒▒▒▒▒▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒  ▒▒  ▒▒▒▒▒▒▒▒▒▒▒  ▒▒▒▒
▓  ▓▓▓   ▓▓       ▓▓▓  ▓▓▓▓  ▓▓       ▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓
█  ████  ██  ███  ███  ████  ██  ████  █████  █████  ████  ██  ████  ██  ███████████  ████
██      ███  ████  ███      ███       ███        ██       ████      ███        ██        █

valid service names:

  processFulltextDocument
  processHeaderDocument
  processReferences
  processCitationList
  processCitationPatentST36
  processCitationPatentPDF

  -O string
        output directory to write parsed files to
  -P    do a ping
  -S string
        server URL (default "http://localhost:8070")
  -c string
        path to config file (default "config.json")
  -cc
        consolidate citations
  -ch
        consolidate header
  -d string
        input directory to scan for PDF, txt, or XML files
  -f string
        single input file to process
  -force
        force reprocess
  -gi
        generate ids
  -ira
        include raw affiliations
  -irc
        include raw citations
  -n int
        number of concurrent workers (default 16)
  -s string
        a valid service name (default "processFulltextDocument")
  -ss
        segment sentences
  -tei
        add pdf coordinates
  -v    be verbose
```

# grobidclient

Go client library and CLI for [grobid](https://github.com/kermitt2/grobid).  To
install the CLI:

```
$ go install github.com/miku/grodidclient/cmd/grobidcli@latest
```

This library includes functions:

* to run parsing on a single PDF file
* to run parsing recursively on a directory of files
* to convert TEI XML to a JSON format, akin to [grobid-tei-xml](https://pypi.org/project/grobid-tei-xml/) (Python, cf. [#41](https://github.com/kermitt2/grobid_client_python/issues/41))

## Notes on server setup

* [Production Grobid Server Configuration](https://github.com/kermitt2/grobid/issues/443#issuecomment-505208132)

## Usage

```shell
$ grobidcli -h
grobidcli | valid service (-s) names:

  processFulltextDocument
  processHeaderDocument
  processReferences
  processCitationList
  processCitationPatentST36
  processCitationPatentPDF

  -H    use sha1 of file contents as the filename
  -O string
        output directory to write parsed files to
  -P    do a ping, then exit
  -S string
        server URL (default "http://localhost:8070")
  -T duration
        client timeout (default 1m0s)
  -W string
        path to WARC file to extract PDFs and parse them (experimental)
  -c string
        path to config file, often config.json
  -cc
        consolidate citations
  -ch
        consolidate header
  -d string
        input directory to scan for PDF, txt, or XML files
  -debug
        use debug result writer, does not create any files
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
        number of concurrent workers (default 12)
  -r int
        max retries (default 10)
  -s string
        a valid service name (default "processFulltextDocument")
  -ss
        segment sentences
  -v    be verbose
  -version
        show version
```

Process a single PDF.

```xml
$ grobidcli -f fixtures/062RoisinAronAmericanNaturalist03.pdf | xmllint --format - | head -10
<?xml version="1.0" encoding="UTF-8"?>
<TEI xmlns="http://www.tei-c.org/ns/1.0" xmlns:xsi="http://www.w3.org/2001/XML...
        <teiHeader xml:lang="en">
                <fileDesc>
                        <titleStmt>
                                <title level="a" type="main">Split Sex Ratios ...
                                <funder ref="#_ZXgvsGF">
                                        <orgName type="full">Belgian National ...
                                </funder>
                        </titleStmt>

...
```

Process pdf files in a directory in parallel.

```shell
$ grobidcli -d fixtures
2024/07/30 20:48:35 scanning fixtures/
2024/07/30 20:48:37 got result [200]: fixtures/62-Article Text-140-1-10-20190621.pdf
2024/07/30 20:48:39 got result [200]: fixtures/062RoisinAronAmericanNaturalist03.pdf
```

By default, for each PDF file a separate file is written to a file with the
`grobid.tei.xml` extension.

## TODO

* [ ] allow to process WARC files
* [ ] allow to group all output from one go into a single file (XML in JSON, really...)

It would be nice to be able to point to a WARC file and parse all found PDFs in
that WARC file.

```shell
$ grobidcli -W https://is.gd/Jpz7OH -o parsed.json
```

* [ ] try to cache processing; cache may be keyed on content hash


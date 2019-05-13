package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"strconv"
)

const (
	// CodeHeader name of the header used as source of the HTTP status code to return
	CodeHeader = "X-Code"

	// ContentType name of the header that defines the format of the reply
	ContentType = "Content-Type"

	// FormatHeader name of the header used to extract the format
	FormatHeader = "X-Format"

	errFilesPath = "www"
)

var errorPages map[string][]byte

func main() {
	errorPages = make(map[string][]byte)
	errorPages["502.html"] = loadErrorPage(502, ".html")
	errorPages["503.html"] = loadErrorPage(502, ".html")

	// 404 is the default
	errorPages["404.html"] = loadErrorPage(502, ".html")

	http.HandleFunc("/", errorHandler(errFilesPath))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":8080"), nil))
}

func loadErrorPage(code int, ext string) []byte {
	file := fmt.Sprintf("%v/%d%v", errFilesPath, code, ext)
	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

func errorHandler(path string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		errCode := r.Header.Get(CodeHeader)
		code, err := strconv.Atoi(errCode)
		if err != nil {
			code = 404
			log.Printf("unexpected error reading return code: %v. Using %v", err, code)
		}

		format := r.Header.Get(FormatHeader)
		if format == "" {
			format = "text/html"
		}

		ext := "html"
		cext, err := mime.ExtensionsByType(format)
		if err != nil {
			log.Printf("unexpected error reading media type extension: %v. Using %v", err, ext)
			format = "text/html"
		} else if len(cext) == 0 {
			log.Printf("couldn't get media type extension. Using %v", ext)
		} else {
			ext = cext[0]
		}

		w.Header().Set(ContentType, format)
		w.WriteHeader(code)

		errorPage := fmt.Sprintf("%d.%s", code, ext)
		if e, ok := errorPages[errorPage]; ok {
			w.Write(e)
		} else {
			log.Printf("couldn't find %s", errorPage)
		}
	}
}

package linguist

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/okteto/okteto/pkg/log"
	enry "gopkg.in/src-d/enry.v1"
)

const (
	readFileLimit = 16 * 1024 * 1024
)

var (
	errAnalysisTimeOut = errors.New("analysis timed out")
)

// this is all based on enry's main command https://github.com/src-d/enry

// ProcessDirectory walks a directory and returns a list of guess for the programming language
func ProcessDirectory(root string) (string, error) {
	out := make(map[string][]string)
	analysisTimeout := false

	timer := time.AfterFunc(5*time.Second, func() {
		analysisTimeout = true
	})

	defer timer.Stop()

	err := filepath.Walk(root, func(path string, f os.FileInfo, inErr error) error {
		if analysisTimeout {
			return errAnalysisTimeOut
		}

		if inErr != nil {
			return inErr
		}

		if !f.Mode().IsDir() && !f.Mode().IsRegular() {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			log.Info(err)
			return nil
		}

		if relativePath == "." {
			return nil
		}

		if f.IsDir() {
			relativePath = relativePath + "/"
		}

		if enry.IsVendor(relativePath) || enry.IsDotFile(relativePath) ||
			enry.IsDocumentation(relativePath) || enry.IsConfiguration(relativePath) {
			if f.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if f.IsDir() {
			return nil
		}

		language, ok := enry.GetLanguageByExtension(path)
		if !ok {
			if language, ok = enry.GetLanguageByFilename(path); !ok {
				content, err := readFile(path, readFileLimit)
				if err != nil {
					log.Info(err)
					return nil
				}

				language = enry.GetLanguage(filepath.Base(path), content)
				if language == enry.OtherLanguage {
					return nil
				}
			}
		}

		if enry.GetLanguageType(language) != enry.Programming {
			return nil
		}

		out[language] = append(out[language], relativePath)
		return nil
	})

	if err != nil && err != errAnalysisTimeOut {
		return Unrecognized, err
	}

	if len(out) == 0 {
		return Unrecognized, nil
	}

	sorted := sortLanguagesByUsage(out)
	if sorted[0] == java {
		if _, err := os.Stat(filepath.Join(root, "pom.xml")); !os.IsNotExist(err) {
			return maven, nil
		}

		return gradle, nil
	}

	return sorted[0], nil
}

func readFile(path string, limit int64) ([]byte, error) {
	if limit <= 0 {
		return ioutil.ReadFile(path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := st.Size()
	if limit > 0 && size > limit {
		size = limit
	}

	buf := bytes.NewBuffer(nil)
	buf.Grow(int(size))
	_, err = io.Copy(buf, io.LimitReader(f, limit))
	return buf.Bytes(), err
}

func sortLanguagesByUsage(fSummary map[string][]string) []string {

	total := 0.0
	keys := make([]string, 0)
	fileValues := make(map[string]float64)

	for fType, files := range fSummary {
		val := float64(len(files))
		fileValues[fType] = val
		keys = append(keys, fType)
		total += val
	}

	sort.Slice(keys, func(i, j int) bool {
		return fileValues[keys[i]] > fileValues[keys[j]]
	})

	// Calculate percentages of each file type.
	var buff bytes.Buffer
	for _, fType := range keys {
		val := fileValues[fType]
		percent := val / total * 100.0
		buff.WriteString(fmt.Sprintf("%.2f%%\t%s\n", percent, fType))
	}

	log.Debugf("Language guesses: \r\n %s", buff.String())

	return keys
}

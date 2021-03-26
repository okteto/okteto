// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	enry "github.com/src-d/enry/v2"
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
			log.Infof("failed to calculate relative path: %w", err)
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
					log.Infof("failed to read %s: %w", path, err)
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
	if len(sorted) == 0 {
		return Unrecognized, nil
	}
	chosen := strings.ToLower(sorted[0])

	if chosen == java {
		return refineJavaChoice(root), nil
	}

	return normalizeLanguage(chosen), nil
}

func refineJavaChoice(root string) string {
	p := filepath.Join(root, "build.gradle")
	_, err := os.Stat(p)
	if err == nil {
		return gradle
	}

	log.Infof("didn't found %s : %s", p, err)
	return maven
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
		if normalizeLanguage(fType) == Unrecognized {
			continue
		}
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
		_, _ = buff.WriteString(fmt.Sprintf("%.2f%%\t%s\n", percent, fType))
	}

	log.Infof("Language guesses: \r\n %s", buff.String())

	return keys
}

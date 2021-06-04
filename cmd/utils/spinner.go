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

package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	sp "github.com/briandowns/spinner"
	"github.com/okteto/okteto/pkg/log"
)

var spinnerSupport bool

//Spinner represents an okteto spinner
type Spinner struct {
	sp *sp.Spinner
}

//NewSpinner returns a new Spinner
func NewSpinner(suffix string) *Spinner {
	spinnerSupport = !loadBoolean("OKTETO_DISABLE_SPINNER")
	s := sp.New(sp.CharSets[14], 100*time.Millisecond)
	s.HideCursor = true
	s.Suffix = fmt.Sprintf(" %s", suffix)
	s.FinalMSG = ""
	return &Spinner{
		sp: s,
	}
}

func loadBoolean(k string) bool {
	v := os.Getenv(k)
	if v == "" {
		v = "false"
	}

	h, err := strconv.ParseBool(v)
	if err != nil {
		log.Yellow("'%s' is not a valid value for environment variable %s", v, k)
	}

	return h
}

//Start starts the spinner
func (p *Spinner) Start() {
	if spinnerSupport {
		p.sp.Start()
	} else {
		fmt.Println(strings.TrimSpace(p.sp.Suffix))
	}
}

//Stop stops the spinner
func (p *Spinner) Stop() {
	if spinnerSupport {
		p.sp.Stop()
	}
}

//Update updates the spinner message
func (p *Spinner) Update(text string) {
	p.sp.Suffix = fmt.Sprintf(" %s", ucFirst(text))
	if !spinnerSupport {
		fmt.Println(strings.TrimSpace(p.sp.Suffix))
	}
}

func ucFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}

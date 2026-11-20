// Copyright 2026 The Okteto Authors
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

package gitcredential

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

const (
	defaultUsername = "x-access-token"
	requestTimeout  = 10 * time.Second
)

var (
	errNoCredentials      = errors.New("the Okteto API returned no token")
	errInvalidCredentials = errors.New("the Okteto API returned a credential with a newline or NUL character")
)

// credentialsGetter asks the Okteto API for the credentials of a repository
type credentialsGetter interface {
	GetGitCredentials(ctx context.Context, host, repository string) (gitCredentials, error)
}

// getterProvider builds the credentialsGetter. It is only called for GitHub requests,
// so other hosts never need OKTETO_GIT_CREDENTIALS_ENDPOINT or OKTETO_TOKEN.
type getterProvider func() (credentialsGetter, error)

// noCredentialsError is returned when the API answers without a token. Its message
// is the API's reason, a sentence meant for the user
type noCredentialsError struct {
	reason string
}

func (e noCredentialsError) Error() string {
	if e.reason == "" {
		return errNoCredentials.Error()
	}
	return e.reason
}

// get answers Git's "get" request. On any failure it prints nothing to stdout,
// so Git moves on as if the helper had no credentials, and writes one line to
// stderr, so the reason shows up in Git's output.
func get(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, provider getterProvider) {
	req, err := readRequest(stdin)
	if err != nil {
		writeFailure(stderr, "", err)
		return
	}
	// Turned off, Git asks the user's own helpers as if Okteto's weren't there
	if !req.isGitHub() || isDisabled() {
		return
	}

	repository, err := req.repository()
	if err != nil {
		writeFailure(stderr, "", err)
		return
	}

	creds, err := fetch(ctx, provider, repository)
	// Outside Okteto's installer and remote runs, Git asks the user's own helpers
	if errors.Is(err, errEndpointNotSet) {
		return
	}
	if err != nil {
		writeFailure(stderr, repository, err)
		return
	}

	if _, err := io.WriteString(stdout, format(creds)); err != nil {
		writeFailure(stderr, repository, err)
	}
}

func fetch(ctx context.Context, provider getterProvider, repository string) (gitCredentials, error) {
	getter, err := provider()
	if err != nil {
		return gitCredentials{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	creds, err := getter.GetGitCredentials(ctx, githubHost, repository)
	if err != nil {
		return gitCredentials{}, err
	}
	if creds.Password == "" {
		return gitCredentials{}, noCredentialsError{reason: creds.Reason}
	}
	if creds.Username == "" {
		creds.Username = defaultUsername
	}
	if hasControlChars(creds.Username) || hasControlChars(creds.Password) {
		return gitCredentials{}, errInvalidCredentials
	}
	return creds, nil
}

// format writes the credentials in Git's credential format. password_expiry_utc
// is a Unix timestamp; Git older than 2.41 ignores it.
func format(creds gitCredentials) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "username=%s\n", creds.Username)
	fmt.Fprintf(&sb, "password=%s\n", creds.Password)
	if !creds.ExpiresAt.IsZero() {
		fmt.Fprintf(&sb, "password_expiry_utc=%d\n", creds.ExpiresAt.Unix())
	}
	return sb.String()
}

// hasControlChars returns true for the characters Git's credential format can't carry
func hasControlChars(s string) bool {
	return strings.ContainsAny(s, "\n\x00")
}

// writeFailure writes the helper's one line to stderr. The reason comes from the
// API or an error, so it is reduced to a single line of printable characters.
func writeFailure(stderr io.Writer, repository string, err error) {
	target := githubHost
	if repository != "" {
		target = repository
	}
	_, _ = fmt.Fprintf(stderr, "okteto: no GitHub credentials for %s: %s\n", target, oneLine(err.Error()))
}

func oneLine(s string) string {
	printable := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(printable), " ")
}

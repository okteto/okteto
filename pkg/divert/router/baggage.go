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

package router

import (
	"net/url"
	"strings"
)

// maxBaggageMembers is the maximum number of list members the W3C Baggage specification
// allows in a single header. Anything past it is ignored so that a pathological header
// cannot turn every request into an unbounded parse.
const maxBaggageMembers = 180

// baggageValue returns the value of the given key in a W3C Baggage header.
//
// The header is a comma-separated list of members, each of them a `key=value` pair
// optionally followed by semicolon-separated properties, with optional whitespace allowed
// around every token:
//
//	key1=value1;property1;property2, key2 = value2, key3=value3; propertyKey=propertyValue
//
// Values are percent-encoded and are returned decoded. A value that fails to decode is
// returned verbatim rather than dropped: a malformed key must degrade to "no divert", never
// to an error.
//
// When a key appears more than once the first occurrence wins, so a downstream hop cannot
// override a routing decision already taken upstream by appending its own member.
// Lookups are case-sensitive, as baggage keys are tokens.
func baggageValue(header, key string) string {
	if header == "" || key == "" {
		return ""
	}

	for i, member := range strings.Split(header, ",") {
		if i >= maxBaggageMembers {
			return ""
		}

		// Properties after the first ';' qualify the member; they are not part of the value.
		member, _, _ = strings.Cut(member, ";")

		name, value, found := strings.Cut(member, "=")
		if !found {
			continue
		}
		if strings.TrimSpace(name) != key {
			continue
		}

		return decodeBaggageValue(strings.TrimSpace(value))
	}

	return ""
}

// decodeBaggageValue percent-decodes a baggage value, falling back to the raw value when it
// is not valid percent-encoding. url.PathUnescape is used rather than url.QueryUnescape
// because baggage is percent-encoded, not form-encoded: a '+' is a literal plus, not a space.
func decodeBaggageValue(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

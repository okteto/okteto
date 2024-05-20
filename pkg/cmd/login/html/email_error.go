// Copyright 2024 The Okteto Authors
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

package html

import "html/template"

const (
	emailErrorTitle = `Business Email Required`

	emailErrorIcon = template.HTML(`
  <svg
    class="illustration"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 330 234"
  >
    <path
      fill-rule="evenodd"
      clip-rule="evenodd"
      d="M316 152.212C316 68.149 248.394 0 165 0S14 68.15 14 152.212c0 22.395 4.84 43.64 13.454 62.788h275.092C311.164 195.852 316 174.607 316 152.212Z"
      fill="#FFF0F0"
    />
    <path
      d="M18 215h294M1 215h10M319 215h10"
      stroke="#E85353"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <path
      fill-rule="evenodd"
      clip-rule="evenodd"
      d="M233 225c0 4.969-29.997 9-67 9s-67-4.031-67-9 29.997-9 67-9 67 4.031 67 9Z"
      fill="#FFF0F0"
    />
    <g fill-rule="evenodd" clip-rule="evenodd" stroke="#E64545">
      <path
        d="m173.257 17 42.506 183.497h14.245a2.998 2.998 0 0 1 2.992 2.992v8.519a2.998 2.998 0 0 1-2.992 2.992s-25.642 7-64.508 7-64.508-7-64.508-7A2.998 2.998 0 0 1 98 212.008v-8.519a2.998 2.998 0 0 1 2.992-2.992h14.245L157.743 17h15.514Z"
        fill="#FFD6D6"
      />
      <path
        d="M198.233 124.957c2.186 9.438 4.736 20.643 6.922 30.081-22.957 15.954-60.899 30.858-87.634 36.107 2.273-9.815 4.51-19.963 6.783-29.778 22.77-6.824 50.869-19.292 69.141-32.751l4.788-3.659ZM195.083 111.018c-22.957 15.955-44.241 25.009-66.26 31.12 2.273-9.816 4.487-19.758 6.76-29.574 20.668-6.665 35.386-13.666 54.019-24.377 0 0 3.294 13.394 5.481 22.831ZM180.876 49.977c1.764 7.617 3.881 16.955 5.645 24.572-11.089 7.219-34.76 17.047-47.018 21.3 2.008-8.67 3.988-16.978 5.996-25.648 8.406-4.075 27.878-14.912 35.377-20.224Z"
        fill="#fff"
      />
    </g>
  </svg>`)

	emailErrorContent = template.HTML(`
<h1 class="error">Business Email Required</h1>
<h2>
  Please add a business/work email to your Github account at
  <a href="https://github.com/settings/emails" target="_blank">
    https://github.com/settings/emails
  </a>
  or try again with a different account.
</h2>`)
)

func emailErrorData() *templateData {
	return &templateData{
		"Content": emailErrorContent,
		"Title":   emailErrorTitle,
		"Icon":    emailErrorIcon,
	}
}

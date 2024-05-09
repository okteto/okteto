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
	successTitle = `Okteto is ready!`

	successIcon = template.HTML(`<svg class="illustration" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 346 244">
  <g fill="none" fill-rule="evenodd">
    <path fill="#F0FDFD" d="M333 158.584C333 71.002 262.26 0 175 0S17 71.002 17 158.584c0 23.332 5.06 45.466 14.078 65.416h287.844C327.94 204.05 333 181.916 333 158.584"/>
    <path stroke="#1DA8B8" stroke-linecap="round" stroke-linejoin="round" d="M18 224h309M1 224h10M335 224h10"/>
    <path fill="#DAF6F5" d="M255.839 209H94.16C86.341 209 80 202.692 80 194.913V88.087C80 80.307 86.341 74 94.161 74H255.84C263.659 74 270 80.308 270 88.087v106.826c0 7.78-6.341 14.087-14.161 14.087"/>
    <path fill="#FFF" d="M261 74H105.653C91.488 74 80 81.235 80 90.157V188c50.85-15.52 81.818-34.52 92.903-57 11.084-22.48 40.45-41.48 88.097-57z"/>
    <path fill="#00D1CA" fill-rule="nonzero" d="M156.23 111.26c14.345-10.537 34.488-7.434 45.463 6.765 2.114 2.734 1.654 6.812-1.31 8.64-2.966 1.826-6.777 1.28-8.846-1.397-5.284-6.836-14.275-9.632-22.464-6.944-8.184 2.686-13.597 10.246-13.417 18.746.18 8.508 5.543 16.028 13.855 18.635 8.303 2.605 17.097-.191 22.095-7.079 2.229-3.072 5.52-3.653 8.65-1.88 3.13 1.773 3.402 6.26 1.467 9.04-5.991 8.257-15.305 13.164-25.722 13.214-10.41.049-20.17-4.635-26.496-12.823-10.975-14.199-7.619-34.378 6.725-44.916zm42.107 19.328a6.016 6.016 0 0 1 5.988.463 6.087 6.087 0 0 1 2.664 5.4 5.899 5.899 0 0 1-3.326 4.961 6.016 6.016 0 0 1-5.988-.463 6.087 6.087 0 0 1-2.664-5.4 5.899 5.899 0 0 1 3.326-4.961z"/>
    <path fill="#FFF" d="M261 74H105.653C91.488 74 80 81.235 80 90.157V188c50.85-15.52 81.818-34.52 92.903-57 11.084-22.48 40.45-41.48 88.097-57z" opacity=".579"/>
    <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M255.839 209H94.16C86.341 209 80 202.692 80 194.913V88.087C80 80.307 86.341 74 94.161 74H255.84C263.659 74 270 80.308 270 88.087v106.826c0 7.78-6.341 14.087-14.161 14.087z"/>
    <path fill="#FFF" d="M191 233h-32l2.667-23h26.666z"/>
    <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M191 233h-32l2.667-23h26.666z"/>
    <path fill="#FFF" d="M210 243h-69c0-6.074 4.75-11 10.615-11h47.77c5.861 0 10.615 4.926 10.615 11M253.714 213c8.997 0 16.286-7.725 16.286-17.25V190H80v5.75c0 9.525 7.29 17.25 16.286 17.25h157.428z"/>
    <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M253.714 213c8.997 0 16.286-7.725 16.286-17.25V190H80v5.75c0 9.525 7.29 17.25 16.286 17.25h157.428zM210 243h-69c0-6.074 4.75-11 10.615-11h47.77c5.861 0 10.615 4.926 10.615 11z"/>
    <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M179 200.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z"/>
    <g>
      <path fill="#FFF" d="M291 85.502C291 105.106 274.877 121 254.998 121 235.118 121 219 105.106 219 85.502 219 65.894 235.118 50 254.998 50S291 65.894 291 85.502"/>
      <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M291 85.502C291 105.106 274.877 121 254.998 121 235.118 121 219 105.106 219 85.502 219 65.894 235.118 50 254.998 50S291 65.894 291 85.502z"/>
      <path fill="#BFF0EE" d="M249.411 106a9.238 9.238 0 0 1-6.486-2.633l-12.234-12.005a8.866 8.866 0 0 1 0-12.723c3.583-3.514 9.39-3.514 12.972 0l5.748 5.638 14.925-14.638c3.582-3.519 9.389-3.519 12.977 0a8.88 8.88 0 0 1 0 12.728l-21.415 21A9.23 9.23 0 0 1 249.41 106"/>
      <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M249.411 106a9.238 9.238 0 0 1-6.486-2.633l-12.234-12.005a8.866 8.866 0 0 1 0-12.723c3.583-3.514 9.39-3.514 12.972 0l5.748 5.638 14.925-14.638c3.582-3.519 9.389-3.519 12.977 0a8.88 8.88 0 0 1 0 12.728l-21.415 21A9.23 9.23 0 0 1 249.41 106z"/>
    </g>
  </g>
  </svg>`)

	successContent = template.HTML(`<h1>You are now logged in!</h1>
  <h2>Your session is now active in the Okteto CLI.<br>Close this window and go back to your terminal</h2>`)
)

func successData() *templateData {
	return &templateData{
		"Content": successContent,
		"Title":   successTitle,
		"Icon":    successIcon,
	}
}

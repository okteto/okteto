// Copyright 2022 The Okteto Authors
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

package login

var loginHTMLTemplate = `<!DOCTYPE html>
<html>
  <head>
    <meta {{.Meta}}>
	<title>Okteto is ready!</title>
	<link rel="shortcut icon" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAAXNSR0IArs4c6QAABJdJREFUWEe9l1tsVFUUhr99znQYOjNQaZEq8gRqxRZ7UywQMUGLQS6+YCI8iDH6IBBJ1agvJuCDJMYLBo3RgC9qNCYGaaNigAgSCy2toW0spfIgCFZ7ox2mnZnOnGX2OVMo07lCcb/utfb+97r8+1+KbJaISceJIiyjAqEGqESpO0HmOe7qPCLdGLQiNGJYv1G6uA+lYpmOV2kNRBTtLXeDtQXhCUSKUZiQyk1AiKFUD4p9YOymrKoLpSTVPakBHDvmZ4bnOUReArk900uS76uLKPUOw6FPWbYskMwmOYCWXxfgcu0GVgCu67s87iVEUXIIw9xMWfXZxLMmA2hrrUFie0DuuaGLJzlLJyrvWRZVNk7cuhZA+8n5WFI/9ZePXymdGOaaiZG4CuD0MT9h9zfAyowvj4xhBkdRoQi66sSdh+XLR6a5M7qCHCAwtn68JhwAutpPNdeh2Jku58bQZTxNHUxvPIXrYi/G4LDj7s8nWlxEuKKEkaXlxOYUgkpR37omDONVyqre093hWLWdLEHkUMpqF8HT8jv+r34k71yPBpzypbHCAoKrHyJYW2NHJk13rGBR9WmFJpn25l0Im5Mai5B/8AQzPm/AGAlNNtFPSMAjLpNg7RICG1elBqH4kLL7X1S0H5+DZbQAc5MB8Jxop+Cjr69erhSWdzrh0gVE581BTBPXP/1Ma+/G7B+6Eh0xDYY3riK45uFU0bqAYVUpTjU9hkgDSpmJljrHRds/xnXhX2fLUISqFjK88XGixYVgxl0sC10fvv0/4/2pERUZs81HF5cx+PLTyQGIaMZcrWhr2o7IG8no1fv9L8z87DvnAKUIVS9kcMtTSL4n+aGxGL76o/gajiCGwdAz6wjV3JciApq21Q4dgXpg9SSGioxR+NYe3B1/2FuW30vfm5uJzr01fatZFq6efhtwtHgWKCO1vdCgAXQBdyVamQNDzH5t15VWG11azuDWDWCmOTB+iApH8O07jPvMOUaXVzOyrByMZH6qSwMIAvmJAFx/9zJ729soy7K3Ak/WElhfm/718V3P8TZuef8LVCyGVeCnd+c2YoUzk/mO5ABgJYH1j94UAClTUPT6LswBh+1ySkEkgu/bw7i7/4ynoCJtCpIWIboId+61+9suwhle+nZkW4R9um2I3qYpOVMRTnUbNhzFV++04fCmtYwuKU/dhsppwxyJ6F6b4SYR0aUAvv1H8B5sRIUdIgo9UMrAK5syENF1UXE+4dL5V6m4p49pHWcx+y9dS8UbVhFcm4mK9WfU1vwB8EJSqDf9M9K3ZvMdt3bi//IH8s6n/46tWQVcXrec4CMPZvkdawBakHQ012FlIUiatSBpcwTJwJAdNPF77ZoIl5fYrJdWkKBFaqIg0afkKMmM4CjGFUnmtr9o8WQjyThAIJIgycaT78hxzQslWVFezkaqk+jYWqqWOD+czRaJy5bl0b1TD0J1oswMsvzaSEzNYKJzDocwjCwHk3EQuibG3M9jqbobGs1E3sUT+YSSXEazcRDjw6lYW4F1/+9wOrE2Jo7nyqhBrMq4iLkjbvYXcAaV+3j+H75mQEXNRGPoAAAAAElFTkSuQmCC" />
    <style type="text/css" media="screen">
      body {
        background-color: #ffffff;
        margin: 0;
        padding: 0;
        font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
      }

      .header {
        margin: 52px;
      }

      .header .logo {
        display: block;
        margin: 0 auto;
      }

      .container {
        margin: 50px auto 40px auto;
        width: 600px;
        text-align: center;
      }

      .container .illustration {
        display: block;
        margin: 0 auto;
      }

      h1 {
        font-size: 42px;
        font-weight: 400;
        color: #00D1CA;
        letter-spacing: -1.2px;
        margin-bottom: 0;
      }

      h2 {
        font-size: 24px;
        font-weight: 300;
        color: #5a6071;
        letter-spacing: -0.1px;
        margin: 12px 0;
      }
    </style>
  </head>
  <body>
    <div class="header">
      <svg class="logo" xmlns="http://www.w3.org/2000/svg" fill="none" width="140" viewBox="0 0 420 121">
        <path fill="#00D1CA" d="M60.5 121a60.5 60.5 0 1 0 0-121 60.5 60.5 0 0 0 0 121Z"/>
        <path fill="#1A263E" d="M39.46 60.5c0-11.52 9.52-21.04 21.49-21.04a21.6 21.6 0 0 1 16.84 7.97 6.76 6.76 0 1 0 10.51-8.52 35.14 35.14 0 0 0-27.35-12.98c-19.24 0-35.02 15.38-35.02 34.57 0 19.2 15.78 34.57 35.02 34.57A35.14 35.14 0 0 0 88.3 82.1a6.76 6.76 0 0 0-10.5-8.52 21.6 21.6 0 0 1-16.85 7.97c-11.97 0-21.5-9.52-21.5-21.04Z"/>
        <path fill="#1A263E" d="M88.3 67.26a6.76 6.76 0 1 0 0-13.52 6.76 6.76 0 0 0 0 13.52ZM227.98 89l-17.25-25.22V89h-9.6V10h9.6v47.9L227.98 33h12.52L221 60l19.5 29h-12.52Zm38.91 0c-4.19 0-7.66-1.33-10.42-4-2.76-2.65-4.17-6.35-4.24-11.1V42.06h-9.01V33h9V19.72h9.64V33h12.57v9.06h-12.57v28.6c0 3.53.67 5.92 2 7.18a6.38 6.38 0 0 0 4.5 1.88 10 10 0 0 0 4.29-.97l3.04 8.85a25.2 25.2 0 0 1-8.8 1.4Zm36.61-56c8 0 13.67 2.68 19.1 7.65 5.43 4.96 8.97 13.11 8.33 22.6h-43.69c.15 5.21 1.9 8.18 5.28 11.36a17.04 17.04 0 0 0 12.11 4.77c6.46 0 11.86-3.35 14.57-9.55h10.65c-1.44 6.34-6.7 12.7-11.02 15.37-4.32 2.67-9.16 3.8-14.59 3.8-7.92 0-14.45-2.68-19.6-8.03-5.13-5.35-7.75-12.62-7.75-20.79 0-8.1 2.47-13.71 7.46-19.1 4.99-5.39 11.37-8.08 19.15-8.08Zm88.84.1c15.28 0 27.66 12.5 27.66 27.95C420 76.48 407.62 89 392.34 89c-15.27 0-27.66-12.52-27.66-27.95 0-15.44 12.38-27.96 27.66-27.96Zm-223.68 0c15.28 0 27.66 12.5 27.66 27.95 0 15.43-12.38 27.95-27.66 27.95S141 76.48 141 61.05c0-15.44 12.38-27.96 27.66-27.96Zm182.2-13.38V33h12.56v9.06h-12.56v28.6c0 3.53.66 5.92 1.99 7.18a6.38 6.38 0 0 0 4.5 1.88 10 10 0 0 0 4.29-.97l3.04 8.85a25.2 25.2 0 0 1-8.8 1.4c-4.19 0-7.66-1.33-10.42-4-2.76-2.65-4.17-6.35-4.24-11.1V42.06h-9V33h9V19.72h9.63Zm41.48 23.3a17.93 17.93 0 0 0-17.84 18.03c0 9.95 7.99 18.02 17.84 18.02s17.84-8.07 17.84-18.02c0-9.96-7.99-18.03-17.84-18.03Zm-223.68 0a17.93 17.93 0 0 0-17.84 18.03c0 9.95 7.99 18.02 17.84 18.02S186.5 71 186.5 61.05c0-9.96-7.99-18.03-17.84-18.03Zm135.55-1.06c-3.85 0-7.66 1.44-10.44 3.77-2.78 2.34-5.21 5.48-6.1 9.42h33c-.79-3.6-2.76-6.9-5.9-9.42-3.14-2.5-5.9-3.77-10.56-3.77Z"/>
      </svg>
    </div>
    <div class="container">
      <svg class="illustration" xmlns="http://www.w3.org/2000/svg" width="346" height="244" viewBox="0 0 346 244">
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
      </svg>
      <h1>You are now logged in!</h1>
      <h2>Your session is now active in {{.Origin}}</h2>
      <h2>{{.Message}}</h2>
    </div>
  </body>
</html>`

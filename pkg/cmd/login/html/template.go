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

var commonTemplate = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <link rel="shortcut icon" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAAXNSR0IArs4c6QAABJdJREFUWEe9l1tsVFUUhr99znQYOjNQaZEq8gRqxRZ7UywQMUGLQS6+YCI8iDH6IBBJ1agvJuCDJMYLBo3RgC9qNCYGaaNigAgSCy2toW0spfIgCFZ7ox2mnZnOnGX2OVMo07lCcb/utfb+97r8+1+KbJaISceJIiyjAqEGqESpO0HmOe7qPCLdGLQiNGJYv1G6uA+lYpmOV2kNRBTtLXeDtQXhCUSKUZiQyk1AiKFUD4p9YOymrKoLpSTVPakBHDvmZ4bnOUReArk900uS76uLKPUOw6FPWbYskMwmOYCWXxfgcu0GVgCu67s87iVEUXIIw9xMWfXZxLMmA2hrrUFie0DuuaGLJzlLJyrvWRZVNk7cuhZA+8n5WFI/9ZePXymdGOaaiZG4CuD0MT9h9zfAyowvj4xhBkdRoQi66sSdh+XLR6a5M7qCHCAwtn68JhwAutpPNdeh2Jku58bQZTxNHUxvPIXrYi/G4LDj7s8nWlxEuKKEkaXlxOYUgkpR37omDONVyqre093hWLWdLEHkUMpqF8HT8jv+r34k71yPBpzypbHCAoKrHyJYW2NHJk13rGBR9WmFJpn25l0Im5Mai5B/8AQzPm/AGAlNNtFPSMAjLpNg7RICG1elBqH4kLL7X1S0H5+DZbQAc5MB8Jxop+Cjr69erhSWdzrh0gVE581BTBPXP/1Ma+/G7B+6Eh0xDYY3riK45uFU0bqAYVUpTjU9hkgDSpmJljrHRds/xnXhX2fLUISqFjK88XGixYVgxl0sC10fvv0/4/2pERUZs81HF5cx+PLTyQGIaMZcrWhr2o7IG8no1fv9L8z87DvnAKUIVS9kcMtTSL4n+aGxGL76o/gajiCGwdAz6wjV3JciApq21Q4dgXpg9SSGioxR+NYe3B1/2FuW30vfm5uJzr01fatZFq6efhtwtHgWKCO1vdCgAXQBdyVamQNDzH5t15VWG11azuDWDWCmOTB+iApH8O07jPvMOUaXVzOyrByMZH6qSwMIAvmJAFx/9zJ729soy7K3Ak/WElhfm/718V3P8TZuef8LVCyGVeCnd+c2YoUzk/mO5ABgJYH1j94UAClTUPT6LswBh+1ySkEkgu/bw7i7/4ynoCJtCpIWIboId+61+9suwhle+nZkW4R9um2I3qYpOVMRTnUbNhzFV++04fCmtYwuKU/dhsppwxyJ6F6b4SYR0aUAvv1H8B5sRIUdIgo9UMrAK5syENF1UXE+4dL5V6m4p49pHWcx+y9dS8UbVhFcm4mK9WfU1vwB8EJSqDf9M9K3ZvMdt3bi//IH8s6n/46tWQVcXrec4CMPZvkdawBakHQ012FlIUiatSBpcwTJwJAdNPF77ZoIl5fYrJdWkKBFaqIg0afkKMmM4CjGFUnmtr9o8WQjyThAIJIgycaT78hxzQslWVFezkaqk+jYWqqWOD+czRaJy5bl0b1TD0J1oswMsvzaSEzNYKJzDocwjCwHk3EQuibG3M9jqbobGs1E3sUT+YSSXEazcRDjw6lYW4F1/+9wOrE2Jo7nyqhBrMq4iLkjbvYXcAaV+3j+H75mQEXNRGPoAAAAAElFTkSuQmCC" />
    <style type="text/css" media="screen">
      body {
        background-color: #ffffff;
        margin: 0;
        padding: 0;
        font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
        display: grid;
        height: 100vh;
        width: 100%;
        grid-template-rows: 140px 1fr;
        place-items: center;
      }

      .container {
        margin: 0 auto 70px;
        width: 92%;
        max-width: 800px;
        text-align: center;
      }

      .container .illustration {
        display: block;
        margin: 0 auto;
        width: 330px;
        max-width: 75%;
      }

      h1 {
        font-size: 42px;
        font-weight: 400;
        color: #00D1CA;
        letter-spacing: -1.2px;
        margin: 32px 0 0;
      }

      h1.error {
        color: #d0021b;
      }

      h2 {
        font-size: 22px;
        line-height: 28px;
        font-weight: 300;
        color: #5a6071;
        margin: 32px 0 0;
      }

      a {
        color: #00d1ca;
        text-decoration: none;
      }

      a:hover {
        text-decoration: underline;
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
      {{.Icon}}
      {{.Content}}
    </div>
  </body>
</html>`

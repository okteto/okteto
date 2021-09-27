// Copyright 2021 The Okteto Authors
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

package okteto

const (
	// DevRegistry fake url for okteto registry
	DevRegistry = "okteto.dev"

	// GlobalRegistry fake url for okteto global registry
	GlobalRegistry = "okteto.global"

	// DefaultGlobalNamespace namespace where okteto app is running
	DefaultGlobalNamespace = "okteto"

	// CloudURL is the default URL of okteto
	CloudURL = "https://cloud.okteto.com"

	// CloudRegistryURL is the default URL of okteto registry
	CloudRegistryURL = "registry.cloud.okteto.net"

	// CloudBuildKitURL is the default URL of okteto buildkit
	CloudBuildKitURL = "tcp://buildkit.cloud.okteto.net:1234"

	// CloudBuildKitCert is the default certificate of okteto buildkit
	CloudBuildKitCert = `-----BEGIN CERTIFICATE-----
	MIIFXDCCBESgAwIBAgISAx7qhpx73g7q1ySy0GcXTdR1MA0GCSqGSIb3DQEBCwUA
	MEoxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MSMwIQYDVQQD
	ExpMZXQncyBFbmNyeXB0IEF1dGhvcml0eSBYMzAeFw0yMDAxMjIwNTI2MDlaFw0y
	MDA0MjEwNTI2MDlaMB0xGzAZBgNVBAMMEiouY2xvdWQub2t0ZXRvLm5ldDCCASIw
	DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANItJ60ucjOfenlAdddCGWRAOu0s
	aQAPLNiV8VNO55jwbJ6m9wA6pqehmWDrd25iMD5xkwepOPfwe7BS8606aniS0c6C
	dzAaX78cTHxTqPreH6szHrXSFs7X2jZd8MhArgq2Ycv7OLX6Zuvi5n+OQq38trn5
	j2s9Nfte55wVkyQAezmuLkvfuXo/5Zopv0L78KnZGkSATaAOEUKAGBjWAX8hQGGV
	fEjfSIjlcH0BnV9kUwH6K333PcyrHnWRzO1airikvfWyyLoX9ZVyFyGycozXdzXH
	FeK5JBlbog2UQ7+s481/Dk5r25U0Eh9/NBxl+QS1PW1AtHnKPfQHjk3iOVMCAwEA
	AaOCAmcwggJjMA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggrBgEFBQcDAQYI
	KwYBBQUHAwIwDAYDVR0TAQH/BAIwADAdBgNVHQ4EFgQU6dKPb56bM17YjU/sx2+4
	yZW3/3wwHwYDVR0jBBgwFoAUqEpqYwR93brm0Tm3pkVl7/Oo7KEwbwYIKwYBBQUH
	AQEEYzBhMC4GCCsGAQUFBzABhiJodHRwOi8vb2NzcC5pbnQteDMubGV0c2VuY3J5
	cHQub3JnMC8GCCsGAQUFBzAChiNodHRwOi8vY2VydC5pbnQteDMubGV0c2VuY3J5
	cHQub3JnLzAdBgNVHREEFjAUghIqLmNsb3VkLm9rdGV0by5uZXQwTAYDVR0gBEUw
	QzAIBgZngQwBAgEwNwYLKwYBBAGC3xMBAQEwKDAmBggrBgEFBQcCARYaaHR0cDov
	L2Nwcy5sZXRzZW5jcnlwdC5vcmcwggEEBgorBgEEAdZ5AgQCBIH1BIHyAPAAdgDw
	laRZ8gDRgkAQLS+TiI6tS/4dR+OZ4dA0prCoqo6ycwAAAW/L7fzmAAAEAwBHMEUC
	IQCqnPlA1iJ0ZE8USqNk/YJQB51FwAasciITaw8yoHr0UwIgdZdlfkscklblwN7d
	m7QfTI08XimtFL1CVbX26uaryVsAdgAHt1wb5X1o//Gwxh0jFce65ld8V5S3au68
	YToaadOiHAAAAW/L7f0NAAAEAwBHMEUCIQCcZr4Q/IkD541Kg4KNEAWVgF6PthZA
	Mh1Zy6o8Qo6jhgIgKo9u41MoSpzZbKNw+0PS0mqAyC169ORnNgehUiJl95cwDQYJ
	KoZIhvcNAQELBQADggEBAHj3xcXRlecO+BbRoQcX1hKqFinaio4WwHQ0D7sLU0sm
	Ye+oIwNuXsuyUNgXPQtm/5CGxPT5Uq09wZEmGasCF272nmxovXac3PaA5WmWV3gs
	a2a8UdG+uqpZZA50G5PhJZSyhOAXWhYq/NqgZICUza3W8UZQ0WvT3os2LZ2bqGif
	ksu0+R3h4FU0KTSI0+9vfaBW2BEVG7jtJxviofF6PZl0zqCQnnWNtOA9rg1rDGq6
	SyRwqhRrQtBBNR8iyHDuVAGrc6v1dhPdxUak1VpgeYVoFNR3ICR7sm0Qy6WVBVjJ
	pE1wqLC3rK2TKzt2dCd4rEEg+WaUckoaALIRonUr7QI=
	-----END CERTIFICATE-----
	`
)

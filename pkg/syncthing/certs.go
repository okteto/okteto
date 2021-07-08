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

package syncthing

var cert = []byte(`-----BEGIN CERTIFICATE-----
MIIBmjCCASCgAwIBAgIIbFSFUVxmIcYwCgYIKoZIzj0EAwMwFDESMBAGA1UEAxMJ
c3luY3RoaW5nMB4XDTE4MTAxNjEyNTE0M1oXDTQ5MTIzMTIzNTk1OVowFDESMBAG
A1UEAxMJc3luY3RoaW5nMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEOdigoiqO0Dzs
RSpSuI+2HvsvFlkD8Knzq/17winWqIy2hltf7hsIiCT9c6gcwKnCcKVJzobXULOn
SlUCXFamReYJ+UEnSpwBprHAECg8fd6+7ctO7O91Cwd/d/ga3IIJoz8wPTAOBgNV
HQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1Ud
EwEB/wQCMAAwCgYIKoZIzj0EAwMDaAAwZQIxAJfWY4sDHCcsbAbLNTr1eTPtbBRd
5Ddo7tDPq6nE/J9R10LG4ia410wK3mN+MfJneQIwOR/xWUO3UjQrGz7jfo7/xFIe
zSXyeq5oAbCfNaByL6e8J5FlC7zUzgYenk7XQmuh
-----END CERTIFICATE-----`)

var key = []byte(`
-----BEGIN EC PRIVATE KEY-----
MIGkAgEBBDD0knlo/G5q0/8YgsW93EOwGqSZyx8I2nXjM9roSrdJgdyLDGJr6s6S
KrfgeVFSfYOgBwYFK4EEACKhZANiAAQ52KCiKo7QPOxFKlK4j7Ye+y8WWQPwqfOr
/XvCKdaojLaGW1/uGwiIJP1zqBzAqcJwpUnOhtdQs6dKVQJcVqZF5gn5QSdKnAGm
scAQKDx93r7ty07s73ULB393+Brcggk=
-----END EC PRIVATE KEY-----`)

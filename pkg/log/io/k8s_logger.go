// Copyright 2023 The Okteto Authors
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

package io

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

const (
	OktetoK8sLoggerEnabledEnvVar = "OKTETO_K8S_REQUESTS_LOGGER_ENABLED"
	K8sLogsFileName              = "okteto-k8s.log"
)

type K8sLogger struct {
	*Controller
}

// NewK8sLogger creates a new k8s logger
func NewK8sLogger() *K8sLogger {
	k8sLogger := &K8sLogger{
		Controller: NewIOController(),
	}
	return k8sLogger
}

// IsEnabled returns true if the k8s logger is enabled
func (k *K8sLogger) IsEnabled() bool {
	return k.loadBooleanOrDefault(OktetoK8sLoggerEnabledEnvVar, false)
}

func (k *K8sLogger) loadBooleanOrDefault(key string, d bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return d
	}

	h, err := strconv.ParseBool(v)
	if err != nil {
		oktetoLog.Yellow("'%s' is not a valid value for environment variable %s", v, k)
	}

	return h
}

// Start configures the k8s logger to write to file
func (k *K8sLogger) Start(okHome, cmdName, flags string) {
	k8sLogsFilepath := GetK8sLoggerFilePath(okHome)
	k.oktetoLogger = newFileLogger(k8sLogsFilepath)
	cmdExecuted := cmdName
	if flags != "" {
		cmdExecuted = fmt.Sprint(cmdName, " ", flags)
	}
	k.Debugf("running cmd: %s", cmdExecuted)
}

// GetK8sLoggerFilePath returns the path of the okteto k8s logs file
func GetK8sLoggerFilePath(okHome string) string {
	k8sLogsFilepath := filepath.Join(okHome, K8sLogsFileName)

	return k8sLogsFilepath
}

// Log logs the http request and response status code
func (k *K8sLogger) Log(respStatusCode int, reqMethod, reqUrl string) {
	decodedUrl, err := url.QueryUnescape(reqUrl)
	if err == nil {
		reqUrl = decodedUrl
	}
	k.Debugf("%d %7s %s", respStatusCode, reqMethod, reqUrl)
}

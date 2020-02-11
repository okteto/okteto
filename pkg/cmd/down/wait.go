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
package down

import (
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/kubernetes"
)

// WaitForDevPodsTermination waits for up to t seconds for pods to be marked as terminated
func WaitForDevPodsTermination(c kubernetes.Interface, d *model.Dev, t int) {
	interactive := map[string]string{labels.InteractiveDevLabel: d.Name}
	detached := map[string]string{labels.DetachedDevLabel: d.Name}

	wg := &sync.WaitGroup{}

	waitForDevPodsTermination(c, d.Namespace, interactive, wg, t)
	if len(d.Services) > 0 {
		waitForDevPodsTermination(c, d.Namespace, detached, wg, t)
	}

	wg.Wait()
}

func waitForDevPodsTermination(c kubernetes.Interface, namespace string, selector map[string]string, wg *sync.WaitGroup, t int) {
	wg.Add(1)
	defer wg.Done()

	t := time.NewTicker(1 * time.Second)
	for i := 0; i < t; i++ {
		ps, err := pods.ListBySelector(namespace, selector, c)
		if err != nil {
			log.Infof("failed to get dev pods with selector %s, exiting: %s", selector, err)
			return
		}

		exit := true
		for i := range ps {
			log.Infof("waiting for %s/%s to terminate", ps[i].GetNamespace(), ps[i].GetName())
			if pods.Exists(ps[i].GetName(), ps[i].GetNamespace(), c) {
				exit = false
			}
		}

		if exit {
			return
		}
		<-t.C
	}
}

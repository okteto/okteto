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
package kubetoken

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	authenticationv1 "k8s.io/api/authentication/v1"
)

var errCacheIsCorrupted = fmt.Errorf("cache is corrupted")

type stringStore interface {
	Get() ([]byte, error)
	Set([]byte) error
}

type CacheGetSetter interface {
	cacheGetter
	cacheSetter
}

type cacheGetter interface {
	Get(contextName, namespace string) (string, error)
}

type cacheSetter interface {
	Set(contextName, namespace string, token authenticationv1.TokenRequest)
}

// Cache is a cache that stores the kubetoken with the underlying store. It handles expiration
type Cache struct {
	StringStore stringStore
	Now         func() time.Time
	Debug       func(string, ...interface{})
}

// NewCache returns a new cache which uses the given file as the underlying store
func NewCache(fileName string) *Cache {
	return &Cache{
		StringStore: NewFileByteStore(fileName),
		Now:         time.Now,
		Debug:       oktetoLog.Debugf,
	}
}

func (c *Cache) read() (storeRegistry, error) {
	contents, err := c.StringStore.Get()
	if err != nil {
		return nil, fmt.Errorf("error while trying to get kubetoken cache: %w", err)
	}

	if len(contents) == 0 {
		return make(storeRegistry), nil
	}

	store := storeRegistry{}

	if err := json.Unmarshal(contents, &store); err != nil {
		return make(storeRegistry), errCacheIsCorrupted
	}

	return store, nil
}

func (c *Cache) Get(contextName, namespace string) (string, error) {
	store, err := c.read()
	if err != nil {
		return "", fmt.Errorf("error while reading: %w", err)
	}

	register, ok := store[key{contextName, namespace}]
	if !ok {
		return "", nil
	}

	now := c.Now()
	if register.Token.Status.ExpirationTimestamp.Time.After(now) {
		tokenString, _ := json.MarshalIndent(register.Token, "", "\t")

		return string(tokenString), nil
	} else {
		// This expired token should get overwritten later on a successful request
		// We won't delete it here so we reduce the number of times we open the file
		return "", nil
	}
}

func updateStore(store storeRegistry, contextName, namespace string, token authenticationv1.TokenRequest) {
	store[key{contextName, namespace}] = storeRegister{
		Token: token,
	}
}

func (c *Cache) setWithErr(contextName, namespace string, token authenticationv1.TokenRequest) error {
	store, err := c.read()
	if err != nil && !errors.Is(err, errCacheIsCorrupted) {
		return err
	}

	updateStore(store, contextName, namespace, token)

	newStore, err := json.MarshalIndent(store, "", "\t")
	if err != nil {
		return err
	}

	return c.StringStore.Set(newStore)
}

func (c *Cache) Set(contextName, namespace string, token authenticationv1.TokenRequest) {
	if err := c.setWithErr(contextName, namespace, token); err != nil {
		c.Debug("failed to write kubetoken cache: %w", err)
	}
}

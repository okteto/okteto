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

type Cache struct {
	StringStore stringStore
	Now         func() time.Time
	Debug       func(string, ...interface{})
}

func NewCache(fileName string) *Cache {
	return &Cache{
		StringStore: NewFileByteStore(fileName),
		Now:         time.Now,
		Debug:       oktetoLog.Debugf,
	}
}

type storeRegister struct {
	Token authenticationv1.TokenRequest `json:"token"`
}

type key struct {
	ContextName string `json:"context"`
	Namespace   string `json:"namespace"`
}

// storeRegistry is a map of key to storeRegister
type storeRegistry map[key]storeRegister

func (s storeRegistry) MarshalJSON() ([]byte, error) {
	m := map[string]storeRegister{}

	for k, v := range s {
		marshaledKey, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}

		m[string(marshaledKey)] = v
	}

	return json.Marshal(m)
}

func (s storeRegistry) UnmarshalJSON(data []byte) error {
	m := map[string]storeRegister{}

	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	for k, v := range m {
		var key key
		if err := json.Unmarshal([]byte(k), &key); err != nil {
			return err
		}

		s[key] = v
	}

	return nil
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

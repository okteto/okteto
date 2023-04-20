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
	ContextName string                        `json:"context"`
	Namespace   string                        `json:"namespace"`
	Token       authenticationv1.TokenRequest `json:"token"`
}

func (c *Cache) read() ([]storeRegister, error) {
	contents, err := c.StringStore.Get()
	if err != nil {
		return nil, fmt.Errorf("error while trying to get kubetoken cache: %w", err)
	}

	if len(contents) == 0 {
		return []storeRegister{}, nil
	}

	var store []storeRegister

	if err := json.Unmarshal(contents, &store); err != nil {
		return nil, errCacheIsCorrupted
	}

	return store, nil
}

func (c *Cache) Get(contextName, namespace string) (string, error) {
	store, err := c.read()
	if err != nil {
		return "", err
	}

	for _, register := range store {
		if register.ContextName == contextName && register.Namespace == namespace {
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
	}

	return "", nil
}

func updateStore(store []storeRegister, contextName, namespace string, token authenticationv1.TokenRequest) []storeRegister {
	existed := false
	for i, r := range store {
		if r.ContextName == contextName && r.Namespace == namespace {
			store[i].Token = token
			existed = true
		}
	}
	if !existed {
		store = append(store, storeRegister{
			ContextName: contextName,
			Namespace:   namespace,
			Token:       token,
		})
	}

	return store
}

func (c *Cache) setWithErr(contextName, namespace string, token authenticationv1.TokenRequest) error {
	store, err := c.read()
	if err != nil && errors.Is(err, errCacheIsCorrupted) {
		return err
	}

	store = updateStore(store, contextName, namespace, token)

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

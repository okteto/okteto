package kubetoken

import (
	"encoding/json"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
)

type stringStore interface {
	Get() ([]byte, error)
	Set([]byte) error
}

type Cache struct {
	StringStore stringStore
	Now         func() time.Time
}

type storeRegister struct {
	ContextName string                        `json:"context"`
	Namespace   string                        `json:"namespace"`
	Token       authenticationv1.TokenRequest `json:"token"`
}

func (c *Cache) read() ([]storeRegister, error) {
	contents, err := c.StringStore.Get()
	if err != nil {
		return nil, err
	}

	if len(contents) == 0 {
		return []storeRegister{}, nil
	}

	var store []storeRegister

	if err := json.Unmarshal(contents, &store); err != nil {
		return nil, fmt.Errorf("error decoding") // TODO: we should probably delete the file contents
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
				// TODO: we could invalidate this cache here
				return "", nil
			}
		}
	}

	return "", nil
}

func (c *Cache) setWithErr(contextName, namespace string, token authenticationv1.TokenRequest) error {
	store, err := c.read()
	if err != nil {
		return err
	}

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

	newStore, err := json.MarshalIndent(store, "", "\t")
	if err != nil {
		return err
	}

	return c.StringStore.Set(newStore)
}

func (c *Cache) Set(contextName, namespace string, token authenticationv1.TokenRequest) {
	if err := c.setWithErr(contextName, namespace, token); err != nil {
		// TODO: log this
	}
}

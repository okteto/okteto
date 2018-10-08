package ksync

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	"github.com/vapor-ware/ksync/pkg/debug"
	pb "github.com/vapor-ware/ksync/pkg/proto"
)

// SpecList is a list of specs.
type SpecList struct {
	Items map[string]*Spec
}

func (s *SpecList) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *SpecList) Fields() log.Fields {
	return log.Fields{}
}

// Message is used to serialize over gRPC
func (s *SpecList) Message() (*pb.SpecList, error) {
	items := map[string]*pb.Spec{}

	for n, v := range s.Items {
		msg, err := v.Message()
		if err != nil {
			return nil, err
		}
		items[n] = msg
	}

	return &pb.SpecList{
		Items: items,
	}, nil
}

// DeserializeSpecList deserializes gRPC messages into a SpecList struct
func DeserializeSpecList(s *pb.SpecList) (*SpecList, error) {
	items := map[string]*Spec{}

	for n, v := range s.Items {
		msg, err := DeserializeSpec(v)
		if err != nil {
			return nil, err
		}
		items[n] = msg
	}

	return &SpecList{
		Items: items,
	}, nil
}

func allSpecs() (map[string]*Spec, error) {
	items := map[string]*Spec{}

	for _, raw := range cast.ToSlice(viper.Get("spec")) {
		var details SpecDetails
		if err := mapstructure.Decode(raw, &details); err != nil {
			log.Warn("This may be due to config changes. Check Release Notes for any updates")
			return nil, err
		}

		// TODO: validate the spec
		items[details.Name] = NewSpec(&details)
	}

	return items, nil
}

// Update looks at config and updates the SpecList to the latest state on disk,
// cleaning any items that are removed.
func (s *SpecList) Update() error {
	if s.Items == nil {
		s.Items = map[string]*Spec{}
	}

	newItems, err := allSpecs()
	if err != nil {
		return err
	}

	// there are new specs to monitor
	for name, spec := range newItems {
		if _, ok := s.Items[name]; !ok {
			s.Items[name] = spec
		}
	}

	// there have been specs removed
	for name, spec := range s.Items {
		if _, ok := newItems[name]; !ok {
			if err := spec.Cleanup(); err != nil {
				return err
			}
			delete(s.Items, name)
		}
	}

	return nil
}

// Watch is a convenience function to have all the configured specs start
// watching.
func (s *SpecList) Watch() error {
	for _, spec := range s.Items {
		if err := spec.Watch(); err != nil {
			return err
		}
	}

	return nil
}

// Create checks an individual input spec for likeness and duplicates
// then adds the spec into the SpecList
func (s *SpecList) Create(details *SpecDetails, force bool) error {
	if !force {
		if s.Has(details.Name) {
			// TODO: make this into a type?
			return fmt.Errorf("name already exists")
		}

		if s.HasLike(details) {
			return fmt.Errorf("similar spec exists")
		}
	}

	s.Items[details.Name] = NewSpec(details)
	return nil
}

// Delete removes a given spec from the SpecList
func (s *SpecList) Delete(name string) error {
	if !s.Has(name) {
		return fmt.Errorf("does not exist")
	}

	delete(s.Items, name)
	return nil
}

// Save serializes the current SpecList's items to the config file.
func (s *SpecList) Save() error {
	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		home, err := homedir.Dir()
		if err != nil {
			return err
		}

		cfgPath = filepath.Join(home, fmt.Sprintf(".%s.yaml", "ksync"))
	}

	log.WithFields(log.Fields{
		"path": cfgPath,
	}).Debug("writing config file")

	var specs []*SpecDetails
	for _, v := range s.Items {
		specs = append(specs, v.Details)
	}
	viper.Set("spec", specs)

	settings := viper.AllSettings()
	// Workaround for #91
	delete(settings, "image")

	buf, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cfgPath, buf, 0644)
}

// HasLike checks a given spec for deep equivalence against another spec
func (s *SpecList) HasLike(target *SpecDetails) bool {
	targetEq := target.Equivalence()
	for _, spec := range s.Items {
		if reflect.DeepEqual(targetEq, spec.Details.Equivalence()) {
			return true
		}
	}
	return false
}

// Has checks a given spec for simple equivalence against another spec
func (s *SpecList) Has(target string) bool {
	if _, ok := s.Items[target]; ok {
		return true
	}
	return false
}

// Get checks the spec list for a matching spec (by name) and returns that spec
func (s *SpecList) Get(name string) (*Spec, error) {
	items, err := allSpecs()
	if err != nil {
		return nil, err
	}

	for name, spec := range items {
		if _, ok := s.Items[name]; ok {
			return spec, nil
		}
	}

	return nil, fmt.Errorf("no specs matching %s", name)
}

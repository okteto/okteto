package ksync

import (
	"fmt"
	"os"

	"github.com/fatih/structs"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"

	"github.com/vapor-ware/ksync/pkg/debug"
	pb "github.com/vapor-ware/ksync/pkg/proto"
)

var (
	specEquivalenceFields = []string{"Name"}
)

// SpecDetails encapsulates all the configuration required to sync files between
// a local and remote folder.
type SpecDetails struct {
	// Local config
	Name string

	// RemoteContainer Locator
	ContainerName string
	Pod           string
	Selector      []string
	Namespace     string

	// File config
	LocalPath  string
	RemotePath string

	// Reload related options
	Reload bool

	// One-way-sync related options
	LocalReadOnly  bool
	RemoteReadOnly bool
}

func (s *SpecDetails) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *SpecDetails) Fields() log.Fields {
	return debug.StructFields(s)
}

// Message is used to serialize over gRPC
func (s *SpecDetails) Message() (*pb.SpecDetails, error) {
	var result pb.SpecDetails
	if err := mapstructure.Decode(structs.Map(s), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeserializeSpecDetails deserializes gRPC messages into a SpecDetails struct
func DeserializeSpecDetails(s *pb.SpecDetails) (*SpecDetails, error) {
	result := &SpecDetails{
		Name:           s.GetName(),
		ContainerName:  s.GetContainerName(),
		Pod:            s.GetPodName(),
		Selector:       s.GetSelector(),
		Namespace:      s.GetNamespace(),
		LocalPath:      s.GetLocalPath(),
		RemotePath:     s.GetRemotePath(),
		Reload:         s.GetReload(),
		LocalReadOnly:  s.GetLocalReadOnly(),
		RemoteReadOnly: s.GetRemoteReadOnly(),
	}

	return result, nil
}

// IsValid returns an error if the spec is not valid.
func (s *SpecDetails) IsValid() error {

	// Cannot sync files, must do directories.
	fstat, err := os.Stat(s.LocalPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	if fstat != nil && !fstat.IsDir() {
		return fmt.Errorf("local path cannot be a single file, please use a directory")
	}

	return nil
}

// Equivalence returns a set of fields that can be used to compare specs for
// equivalence via. reflect.DeepEqual.
func (s *SpecDetails) Equivalence() map[string]interface{} {
	vals := structs.Map(s)
	for _, k := range specEquivalenceFields {
		delete(vals, k)
	}
	return vals
}

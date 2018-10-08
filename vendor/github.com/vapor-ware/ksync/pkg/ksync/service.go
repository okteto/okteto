package ksync

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/vapor-ware/ksync/pkg/debug"
	pb "github.com/vapor-ware/ksync/pkg/proto"
)

// ServiceStatus is the current status of a service.
type ServiceStatus string

const (
	// ServiceStopped is for when a service is stopped.
	ServiceStopped ServiceStatus = "stopped"
	// ServiceStarting is for when a service is starting.
	ServiceStarting ServiceStatus = "starting"
	// ServiceWatching is for when a service is watching.
	ServiceWatching ServiceStatus = "watching"
	// ServiceUpdating is for when a service is updating.
	ServiceUpdating ServiceStatus = "updating"
	// ServiceReloading is for when the container is restarting.
	ServiceReloading ServiceStatus = "reloading"
	// ServiceError is for when a service is experiencing an error.
	ServiceError ServiceStatus = "error"
)

// Service reflects a spec that will sync files between a local and remote
// folder.
type Service struct {
	RemoteContainer *RemoteContainer
	SpecDetails     *SpecDetails

	folder *Folder
}

// NewService constructs a Service to sync files between a local and remote
// folder.
func NewService(cntr *RemoteContainer, details *SpecDetails) *Service {
	return &Service{
		RemoteContainer: cntr,
		SpecDetails:     details,
	}
}

func (s *Service) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *Service) Fields() log.Fields {
	return s.RemoteContainer.Fields()
}

// ShortFields returns a set of structured fields to show users that are simplified.
func (s *Service) ShortFields() log.Fields {
	return log.Fields{
		"pod":  s.RemoteContainer.PodName,
		"spec": s.SpecDetails.Name,
	}
}

// Message is used to serialize over gRPC
func (s *Service) Message() (*pb.Service, error) {
	cntr, err := s.RemoteContainer.Message()
	if err != nil {
		return nil, err
	}

	details, err := s.SpecDetails.Message()
	if err != nil {
		return nil, err
	}

	return &pb.Service{
		RemoteContainer: cntr,
		SpecDetails:     details,
		Status:          string(s.Status()),
	}, nil
}

// DeserializeService deserializes gRPC messages into a Service struct
func DeserializeService(s *pb.Service) (*Service, error) {
	cntr, err := DeserializeRemoteContainer(s.RemoteContainer)
	if err != nil {
		return nil, err
	}

	details, err := DeserializeSpecDetails(s.SpecDetails)
	if err != nil {
		return nil, err
	}

	return &Service{
		RemoteContainer: cntr,
		SpecDetails:     details,
	}, nil
}

// Status returns the current status of this service.
func (s *Service) Status() ServiceStatus {
	if s.folder == nil {
		return ServiceStopped
	}

	return s.folder.Status
}

// Start runs this service.
func (s *Service) Start() error {
	if s.folder != nil {
		return fmt.Errorf("already running")
	}

	s.folder = NewFolder(s)

	if err := s.folder.Run(); err != nil {
		return err
	}

	log.WithFields(s.ShortFields()).Info("folder sync running")

	return nil
}

// Stop halts a service that has been running in the background.
func (s *Service) Stop() error {
	log.WithFields(s.ShortFields()).Info("stopping")

	log.WithFields(s.Fields()).Debug("stopping service")
	return s.folder.Stop()
}

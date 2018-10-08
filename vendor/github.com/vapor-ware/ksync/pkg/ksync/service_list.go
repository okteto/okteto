package ksync

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vapor-ware/ksync/pkg/debug"
	pb "github.com/vapor-ware/ksync/pkg/proto"
)

// ServiceList is a list of services.
type ServiceList struct {
	Items []*Service
}

func (s *ServiceList) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *ServiceList) Fields() log.Fields {
	return log.Fields{}
}

// Message is used to serialize over gRPC
func (s *ServiceList) Message() (*pb.ServiceList, error) {
	items := []*pb.Service{}

	for _, v := range s.Items {
		msg, err := v.Message()
		if err != nil {
			return nil, err
		}
		items = append(items, msg)
	}

	return &pb.ServiceList{
		Items: items,
	}, nil
}

// DeserializeServiceList deserializes gRPC messages into a ServiceList struct
func DeserializeServiceList(s *pb.ServiceList) (*ServiceList, error) {
	items := []*Service{}

	for _, v := range s.Items {
		msg, err := DeserializeService(v)
		if err != nil {
			return nil, err
		}
		items = append(items, msg)
	}

	return &ServiceList{
		Items: items,
	}, nil
}

// NewServiceList is a constructor for ServiceList
func NewServiceList() *ServiceList {
	return &ServiceList{
		Items: []*Service{},
	}
}

// Add takes a pod/spec, creates a new service, adds it to the list and starts it.
func (s *ServiceList) Add(pod *v1.Pod, details *SpecDetails) error {
	cntr, err := NewRemoteContainer(pod, details.ContainerName)
	if err != nil {
		return err
	}

	service := NewService(cntr, details)
	if s.Has(service) {
		return &errors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Reason:  metav1.StatusReasonAlreadyExists,
				Message: fmt.Sprintf("service for %s already exists", pod.Name),
			}}
	}

	s.Items = append(s.Items, service)

	log.WithFields(log.Fields{
		"pod":  cntr.PodName,
		"spec": service.SpecDetails.Name,
	}).Info("new pod detected")

	log.WithFields(service.Fields()).Debug("added service")

	return service.Start()
}

// Has checks for equivalence between the existing services.
func (s *ServiceList) Has(target *Service) bool {
	for _, service := range s.Items {
		if reflect.DeepEqual(service.RemoteContainer, target.RemoteContainer) {
			return true
		}
	}

	return false
}

// Pop fetches a service by pod name and removes it from the list.
func (s *ServiceList) Pop(podName string) *Service {
	for i, service := range s.Items {
		if service.RemoteContainer.PodName == podName {
			s.Items[i] = s.Items[len(s.Items)-1]
			s.Items = s.Items[:len(s.Items)-1]
			return service
		}
	}

	return nil
}

// Get searches the service list for a matching service (by name) and returns it
func (s *ServiceList) Get(name string) (*Service, error) {
	for _, service := range s.Items {
		if service.SpecDetails.Name == name {
			return service, nil
		}
	}

	return nil, fmt.Errorf("couldn't get service with name %s", name)
}

// Stop takes all the services in a list and stops them.
func (s *ServiceList) Stop() error {
	for _, service := range s.Items {
		if err := service.Stop(); err != nil {
			return err
		}
	}

	return nil
}

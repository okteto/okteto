package ksync

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/vapor-ware/ksync/pkg/debug"
	"github.com/vapor-ware/ksync/pkg/ksync/cluster"
	pb "github.com/vapor-ware/ksync/pkg/proto"
)

// SpecStatus is the status of a spec
type SpecStatus string

// See docs/spec-lifecycle.png
const (
	SpecWaiting SpecStatus = "waiting"
	SpecRunning SpecStatus = "running"
)

// Spec is what manages the configuration and state of a folder being synced
// between the localhost and a remote container. It has a list of services
// for remote containers that match the SpecDetails (active folder syncs).
type Spec struct {
	Details  *SpecDetails
	Services *ServiceList `structs:"-"`

	Status SpecStatus

	stopWatching chan bool
}

func (s *Spec) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *Spec) Fields() log.Fields {
	return s.Details.Fields()
}

// Message is used to serialize over gRPC
func (s *Spec) Message() (*pb.Spec, error) {
	details, err := s.Details.Message()
	if err != nil {
		return nil, err
	}

	services, err := s.Services.Message()
	if err != nil {
		return nil, err
	}

	return &pb.Spec{
		Details:  details,
		Services: services,
		Status:   string(s.Status),
	}, nil
}

// DeserializeSpec deserializes gRPC messages into a Spec struct
func DeserializeSpec(s *pb.Spec) (*Spec, error) {
	details, err := DeserializeSpecDetails(s.GetDetails())
	if err != nil {
		return nil, err
	}

	services, err := DeserializeServiceList(s.GetServices())
	if err != nil {
		return nil, err
	}

	status := SpecStatus(s.GetStatus())
	return &Spec{
		Details:  details,
		Services: services,
		Status:   status,
	}, nil
}

// NewSpec is a constructor for Specs
func NewSpec(details *SpecDetails) *Spec {
	return &Spec{
		Details:  details,
		Services: NewServiceList(),
		Status:   SpecWaiting,
	}
}

// Watch will contact the cluster's api server and start watching for events
// that match the spec. If a match is found, a new service is started up to
// mange syncing the folder. If a event shows that the match is going away,
// the running service is stopped.
func (s *Spec) Watch() error {
	if s.stopWatching != nil {
		log.WithFields(s.Fields()).Debug("already watching")
		return nil
	}

	selectors := strings.Join(s.Details.Selector, ",")
	
	opts := metav1.ListOptions{}
	opts.LabelSelector = selectors
	watcher, err := cluster.Client.CoreV1().Pods(s.Details.Namespace).Watch(opts)
	if err != nil {
		return err
	}

	log.WithFields(s.Fields()).Debug("watching for updates")

	s.stopWatching = make(chan bool)
	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-s.stopWatching:
				log.WithFields(s.Fields()).Debug("stopping watch")
				return

			case event := <-watcher.ResultChan():
				// For whatever reason under the sun, if the watcher looses the
				// connection with the cluster, it ends up sending empty events
				// as fast as possible. We want to just kill this when that's the
				// case.
				if event.Type == "" && event.Object == nil {
					log.WithFields(s.Fields()).Error("lost connection to cluster")
					SignalLoss <- true
					return
				}

				if event.Object == nil {
					continue
				}

				if err := s.handleEvent(event); err != nil {
					log.WithFields(s.Fields()).Error(err)
				}
			}
		}
	}()

	return nil
}

func (s *Spec) handleEvent(event watch.Event) error {
	pod := event.Object.(*v1.Pod)
	if event.Type != watch.Modified && event.Type != watch.Added {
		return nil
	}

	log.WithFields(log.Fields{
		"type":    event.Type,
		"name":    pod.Name,
		"status":  pod.Status.Phase,
		"deleted": pod.DeletionTimestamp != nil,
	}).Debug("new event")

	if pod.DeletionTimestamp != nil {
		return s.cleanService(pod)
	}

	if pod.Status.Phase == v1.PodRunning {
		s.Status = SpecRunning
		return s.addService(pod)
	}

	return nil
}

func (s *Spec) addService(pod *v1.Pod) error {
	if err := s.Services.Add(pod, s.Details); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func (s *Spec) cleanService(pod *v1.Pod) error {
	service := s.Services.Pop(pod.Name)
	if service == nil {
		log.WithFields(s.Fields()).Debug("service not found")
		return nil
	}

	if err := service.Stop(); err != nil {
		return err
	}

	if len(s.Services.Items) == 0 {
		s.Status = SpecWaiting
	}

	return nil
}

// Cleanup will remove anything running in the background, meant to be used when
// a spec is deleted.
func (s *Spec) Cleanup() error {
	if s.stopWatching != nil {
		close(s.stopWatching)
	}

	return s.Services.Stop()
}

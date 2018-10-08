package cluster

import (
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vapor-ware/ksync/pkg/debug"
	pb "github.com/vapor-ware/ksync/pkg/proto"
)

var (
	// ImageName is the docker image to use for running the cluster service.
	ImageName = "vaporio/ksync"
)

// SetImage sets the package-wide image to used for running the cluster service.
func SetImage(name string) {
	ImageName = name
}

// Service is the remote server component of ksync.
type Service struct {
	Namespace string
	name      string
	labels    map[string]string

	RadarPort         int32
	SyncthingAPI      int32
	SyncthingListener int32
}

func (s *Service) String() string {
	return debug.YamlString(s)
}

// Fields returns a set of structured fields for logging.
func (s *Service) Fields() log.Fields {
	return debug.StructFields(s)
}

// NewService constructs a Service to track the ksync daemonset on the cluster.
func NewService() *Service {
	return &Service{
		Namespace: "kube-system",
		name:      "ksync",
		labels: map[string]string{
			"name": "ksync",
			"app":  "ksync",
		},

		RadarPort:         40321,
		SyncthingAPI:      8384,
		SyncthingListener: 22000,
	}
}

// IsInstalled makes sure the cluster service has been installed.
func (s *Service) IsInstalled() (bool, error) {
	// TODO: add version checking here.
	if _, err := Client.ExtensionsV1beta1().DaemonSets(s.Namespace).Get(
		s.name, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

// PodName takes the name of a node and returns the name of the ksync pod
// running on that node.
func (s *Service) PodName(nodeName string) (string, error) {
	// TODO: error handling for nodes that don't exist.
	pods, err := Client.CoreV1().Pods(s.Namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", s.labels["app"]),
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
		})

	if err != nil {
		return "", err
	}

	// TODO: I don't want to go and setup a whole bunch of error handling code
	// right now. The NotFound error works perfectly here for now.
	if len(pods.Items) == 0 {
		return "", &errors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Reason:  metav1.StatusReasonNotFound,
				Message: fmt.Sprintf("%s not found on %s", s.name, nodeName),
			}}
	}

	if len(pods.Items) > 1 {
		return "", fmt.Errorf(
			"unexpected result looking up radar pod (count:%d) (node:%s)",
			len(pods.Items),
			nodeName)
	}

	return pods.Items[0].Name, nil
}

// IsHealthy verifies the target node is running the service container and it
// is not scheduled for deletion.
func (s *Service) IsHealthy(nodeName string) (bool, error) {
	log.WithFields(log.Fields{
		"nodeName": nodeName,
	}).Debug("checking to see if radar is ready")

	installed, err := s.IsInstalled()
	if err != nil {
		return false, err
	}

	if !installed {
		return false, fmt.Errorf("radar is not installed")
	}

	podName, err := s.PodName(nodeName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return false, debug.ErrorOut("cannot get pod name", err, s)
		}

		return false, nil
	}

	log.WithFields(debug.MergeFields(s.Fields(), log.Fields{
		"nodeName": nodeName,
		"podName":  podName,
	})).Debug("found pod name")

	pod, err := Client.CoreV1().Pods(s.Namespace).Get(
		podName, metav1.GetOptions{})
	if err != nil {
		return false, debug.ErrorOut("cannot get pod details", err, s)
	}

	log.WithFields(log.Fields{
		"podName":  pod.Name,
		"nodeName": nodeName,
		"status":   pod.Status.Phase,
	}).Debug("found pod")

	if pod.Status.Phase != v1.PodRunning || pod.DeletionTimestamp != nil {
		return false, nil
	}

	return true, nil
}

// NodeNames returns a list of all the nodes the ksync pod is running on.
func (s *Service) NodeNames() ([]string, error) {
	result := []string{}

	opts := metav1.ListOptions{}
	opts.LabelSelector = "app=ksync"
	pods, err := Client.CoreV1().Pods(s.Namespace).List(opts)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"count": len(pods.Items),
	}).Debug("radar nodes")

	for _, pod := range pods.Items {
		result = append(result, pod.Spec.NodeName)
	}

	return result, nil
}

// Run starts (or upgrades) the ksync daemonset on the remote cluster.
func (s *Service) Run(upgrade bool) error {
	daemonSets := Client.ExtensionsV1beta1().DaemonSets(s.Namespace)

	if _, err := daemonSets.Create(s.daemonSet()); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	if upgrade {
		if _, err := daemonSets.Update(s.daemonSet()); err != nil {
			return err
		}
	}

	log.WithFields(debug.MergeFields(s.Fields(), log.Fields{
		"upgrade": upgrade,
	})).Debug("started DaemonSet")

	return nil
}

func (s *Service) nodeVersion(nodeName string) (*pb.VersionInfo, error) {
	conn, err := NewConnection(nodeName).Radar()
	if err != nil {
		return nil, err
	}
	defer conn.Close() // nolint: errcheck

	return pb.NewRadarClient(conn).GetVersionInfo(
		context.Background(), &empty.Empty{})
}

// Version fetches the binary version from radar running in the remote cluster.
// It picks the first node that is healthy. If no nodes are healthy, an error
// is thrown.
func (s *Service) Version() (*pb.VersionInfo, error) {
	nodes, err := s.NodeNames()
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		state, healthErr := s.IsHealthy(node)
		if healthErr != nil {
			return nil, healthErr
		}

		if state {
			return s.nodeVersion(node)
		}
	}

	return nil, fmt.Errorf("no healthy nodes found")
}

// Remove provides cleanup for the ksync daemonset on the cluster. Only run this
// when you want to clean everything up.
func (s *Service) Remove() error {
	daemonSets := Client.ExtensionsV1beta1().DaemonSets(s.Namespace)

	if err := daemonSets.Delete(s.name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	log.WithFields(debug.MergeFields(s.Fields(), log.Fields{
		"Name": s.name,
	})).Debug("Removed DaemonSet")

	return nil
}

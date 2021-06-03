package diverts

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type divertServiceModification struct {
	ProxyProtocol      string `json:"proxy_port"`
	OriginalPort       string `json:"original_port"`
	OriginalTargetPort string `json:"original_target_port"`
}

// DivertName returns the name of the diverted version of a given resource
func DivertName(username, name string) string {
	return fmt.Sprintf("%s-%s", username, name)
}

func translateDeployment(username string, d *appsv1.Deployment) *appsv1.Deployment {
	result := d.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, d.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if d.Labels != nil && d.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = d.Labels[model.DeployedByLabel]
	}
	result.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			model.OktetoDivertLabel: username,
		},
	}
	result.Spec.Template.Labels = map[string]string{
		model.OktetoDivertLabel: username,
	}
	annotations.Set(result.GetObjectMeta(), model.OktetoAutoCreateAnnotation, model.OktetoUpCmd)
	result.ResourceVersion = ""
	return result
}

func translateService(username string, d *appsv1.Deployment, s *apiv1.Service) (*apiv1.Service, error) {
	result := s.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, s.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if s.Labels != nil && s.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = s.Labels[model.DeployedByLabel]
	}
	if s.Annotations != nil {
		modification := s.Annotations[model.OktetoDivertServiceModificationAnnotation]
		if modification != "" {
			var dsm divertServiceModification
			if err := json.Unmarshal([]byte(modification), &dsm); err != nil {
				return nil, fmt.Errorf("bad divert service modification: %s", modification)
			}
			for i := range result.Spec.Ports {
				if strings.Compare(fmt.Sprint(result.Spec.Ports[i].Port), dsm.OriginalPort) == 0 {
					portNumber, err := strconv.Atoi(dsm.OriginalTargetPort)
					if err != nil {
						return nil, fmt.Errorf("error parsing port number %s: %s", dsm.OriginalTargetPort, err.Error())
					}
					result.Spec.Ports[i].TargetPort = intstr.FromInt(portNumber)
				}
			}
		}
	}
	delete(result.Annotations, model.OktetoAutoIngressAnnotation)
	delete(result.Annotations, model.OktetoDivertServiceModificationAnnotation)
	result.Spec.Selector = map[string]string{
		model.OktetoDivertLabel:   username,
		model.InteractiveDevLabel: d.Name,
	}
	result.ResourceVersion = ""
	result.Spec.ClusterIP = ""
	return result, nil
}

func translateIngress(username string, i *networkingv1.Ingress) *networkingv1.Ingress {
	result := i.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, i.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if i.Labels != nil && i.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = i.Labels[model.DeployedByLabel]
	}
	if host := annotations.Get(result.GetObjectMeta(), model.OktetoIngressAutoGenerateHost); host != "" {
		if host != "true" {
			annotations.Set(result.GetObjectMeta(), model.OktetoIngressAutoGenerateHost, fmt.Sprintf("%s-%s", username, host))
		}
	} else {
		annotations.Set(result.GetObjectMeta(), model.OktetoIngressAutoGenerateHost, "true")
	}
	result.ResourceVersion = ""
	return result
}

func translateDivertCRD(username string, dev *model.Dev, s *apiv1.Service, i *networkingv1.Ingress) *Divert {
	result := &Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: dev.Namespace,
		},
		Spec: DivertSpec{
			Ingress: IngressDivertSpec{
				Name:      i.Name,
				Namespace: dev.Namespace,
				Value:     username,
			},
			FromService: ServiceDivertSpec{
				Name:      dev.Divert.Service,
				Namespace: dev.Namespace,
				Port:      dev.Divert.Port,
			},
			ToService: ServiceDivertSpec{
				Name:      s.Name,
				Namespace: dev.Namespace,
				Port:      dev.Divert.Port,
			},
			Deployment: DeploymentDivertSpec{
				Name:      dev.Name,
				Namespace: dev.Namespace,
			},
		},
	}
	if s.Labels != nil && s.Labels[model.DeployedByLabel] != "" {
		result.Labels = map[string]string{model.DeployedByLabel: s.Labels[model.DeployedByLabel]}
	}
	return result
}

func translateDev(username string, dev *model.Dev, d *appsv1.Deployment) {
	dev.Name = d.Name
	dev.Labels = nil
}

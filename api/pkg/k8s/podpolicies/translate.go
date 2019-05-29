package podpolicies

import (
	"os"

	"github.com/okteto/app/api/pkg/model"
	v1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	falseValue = false
	trueValue  = true
)

func translate(s *model.Space) *v1beta1.PodSecurityPolicy {
	softMultitenancy := os.Getenv("OKTETO_SOFT_MULTITENANCY")
	if softMultitenancy == "YES" {
		return &v1beta1.PodSecurityPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.ID,
				Annotations: map[string]string{
					"seccomp.security.alpha.kubernetes.io/allowedProfileNames": "docker/default",
					"apparmor.security.beta.kubernetes.io/allowedProfileNames": "runtime/default",
					"seccomp.security.alpha.kubernetes.io/defaultProfileName":  "docker/default",
					"apparmor.security.beta.kubernetes.io/defaultProfileName":  "runtime/default",
				},
			},
			Spec: v1beta1.PodSecurityPolicySpec{
				Privileged:               true,
				AllowPrivilegeEscalation: &trueValue,
				AllowedHostPaths:         []v1beta1.AllowedHostPath{},
				Volumes:                  []v1beta1.FSType{v1beta1.ConfigMap, v1beta1.EmptyDir, v1beta1.Projected, v1beta1.Secret, v1beta1.DownwardAPI, v1beta1.PersistentVolumeClaim},
				HostNetwork:              true,
				HostIPC:                  true,
				HostPID:                  true,
				RunAsUser: v1beta1.RunAsUserStrategyOptions{
					Rule: v1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: v1beta1.SELinuxStrategyOptions{
					Rule: v1beta1.SELinuxStrategyRunAsAny,
				},
				SupplementalGroups: v1beta1.SupplementalGroupsStrategyOptions{
					Rule: v1beta1.SupplementalGroupsStrategyRunAsAny,
				},
				FSGroup: v1beta1.FSGroupStrategyOptions{
					Rule: v1beta1.FSGroupStrategyRunAsAny,
				},
			},
		}

	}
	return &v1beta1.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.ID,
			Annotations: map[string]string{
				"seccomp.security.alpha.kubernetes.io/allowedProfileNames": "docker/default",
				"apparmor.security.beta.kubernetes.io/allowedProfileNames": "runtime/default",
				"seccomp.security.alpha.kubernetes.io/defaultProfileName":  "docker/default",
				"apparmor.security.beta.kubernetes.io/defaultProfileName":  "runtime/default",
			},
		},
		Spec: v1beta1.PodSecurityPolicySpec{
			Privileged:               false,
			AllowPrivilegeEscalation: &falseValue,
			AllowedHostPaths: []v1beta1.AllowedHostPath{
				v1beta1.AllowedHostPath{
					PathPrefix: "/tmp",
					ReadOnly:   true,
				},
			},
			Volumes:     []v1beta1.FSType{v1beta1.ConfigMap, v1beta1.EmptyDir, v1beta1.Projected, v1beta1.Secret, v1beta1.DownwardAPI, v1beta1.PersistentVolumeClaim},
			HostNetwork: false,
			HostIPC:     false,
			HostPID:     false,
			RunAsUser: v1beta1.RunAsUserStrategyOptions{
				Rule: v1beta1.RunAsUserStrategyRunAsAny,
			},
			SELinux: v1beta1.SELinuxStrategyOptions{
				Rule: v1beta1.SELinuxStrategyRunAsAny,
			},
			SupplementalGroups: v1beta1.SupplementalGroupsStrategyOptions{
				Rule: v1beta1.SupplementalGroupsStrategyRunAsAny,
			},
			FSGroup: v1beta1.FSGroupStrategyOptions{
				Rule: v1beta1.FSGroupStrategyRunAsAny,
			},
		},
	}
}

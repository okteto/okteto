// Copyright 2026 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rollouts

import (
	"context"
	"encoding/json"
	"fmt"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/conditions"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

type patchAnnotations struct {
	Value map[string]string `json:"value"`
	Op    string            `json:"op"`
	Path  string            `json:"path"`
}

// List returns the list of rollouts.
func List(ctx context.Context, namespace, labels string, c rolloutsclientset.Interface) ([]rolloutsv1alpha1.Rollout, error) {
	rList, err := c.ArgoprojV1alpha1().Rollouts(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return rList.Items, nil
}

// Get returns a rollout object by name.
func Get(ctx context.Context, name, namespace string, c rolloutsclientset.Interface) (*rolloutsv1alpha1.Rollout, error) {
	return c.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetByDev returns a rollout object given a dev struct (by name or by label).
func GetByDev(ctx context.Context, dev *model.Dev, namespace string, c rolloutsclientset.Interface) (*rolloutsv1alpha1.Rollout, error) {
	if len(dev.Selector) == 0 {
		return Get(ctx, dev.Name, namespace, c)
	}

	rList, err := c.ArgoprojV1alpha1().Rollouts(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: dev.LabelsSelector(),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(rList.Items) == 0 {
		return nil, oktetoErrors.ErrNotFound
	}

	validRollouts := []*rolloutsv1alpha1.Rollout{}
	for i := range rList.Items {
		if rList.Items[i].Labels[model.DevCloneLabel] == "" {
			validRollouts = append(validRollouts, &rList.Items[i])
		}
	}

	if len(validRollouts) == 0 {
		return nil, oktetoErrors.ErrNotFound
	}
	if len(validRollouts) > 1 {
		return nil, fmt.Errorf("found '%d' rollouts for labels '%s' instead of 1", len(validRollouts), dev.LabelsSelector())
	}
	return validRollouts[0], nil
}

// CheckConditionErrors checks errors in conditions.
func CheckConditionErrors(rollout *rolloutsv1alpha1.Rollout, dev *model.Dev) error {
	for _, c := range rollout.Status.Conditions {
		if c.Type == rolloutsv1alpha1.InvalidSpec && c.Status == apiv1.ConditionTrue {
			return fmt.Errorf("%s", c.Message)
		}

		if c.Type == rolloutsv1alpha1.RolloutReplicaFailure && c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
			return conditions.FailedCreateError(c.Message, dev)
		}
	}
	return nil
}

// Deploy creates or updates a rollout.
func Deploy(ctx context.Context, r *rolloutsv1alpha1.Rollout, c rolloutsclientset.Interface) (*rolloutsv1alpha1.Rollout, error) {
	r.ResourceVersion = ""
	result, err := c.ArgoprojV1alpha1().Rollouts(r.Namespace).Update(ctx, r, metav1.UpdateOptions{})
	if err == nil {
		return result, nil
	}

	if !oktetoErrors.IsNotFound(err) {
		return nil, err
	}

	return c.ArgoprojV1alpha1().Rollouts(r.Namespace).Create(ctx, r, metav1.CreateOptions{})
}

// Destroy removes a rollout object given its name and namespace.
func Destroy(ctx context.Context, name, namespace string, c rolloutsclientset.Interface) error {
	oktetoLog.Infof("deleting rollout '%s'", name)
	err := c.ArgoprojV1alpha1().Rollouts(namespace).Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: ptr.To(int64(0))})
	if err != nil {
		if oktetoErrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
			return nil
		}
		return fmt.Errorf("error deleting argo rollout: %w", err)
	}
	oktetoLog.Infof("rollout '%s' deleted", name)
	return nil
}

// PatchAnnotations patches the rollout annotations.
func PatchAnnotations(ctx context.Context, r *rolloutsv1alpha1.Rollout, c rolloutsclientset.Interface) error {
	payload := []patchAnnotations{
		{
			Op:    "replace",
			Path:  "/metadata/annotations",
			Value: r.Annotations,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := c.ArgoprojV1alpha1().Rollouts(r.Namespace).Patch(ctx, r.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{}); err != nil {
		return err
	}
	return nil
}

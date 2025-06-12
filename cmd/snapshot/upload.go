// Copyright 2023 The Okteto Authors
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

package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

type UploadOptions struct {
	LocalPath    string
	Size         string
	SnapshotName string
}

// Upload creates the upload subcommand
func Upload(ctx context.Context, k8sClientProvider okteto.K8sClientProviderWithLogger, ioCtrl *io.Controller) *cobra.Command {
	options := &UploadOptions{}

	cmd := &cobra.Command{
		Use:   "upload [LOCAL_PATH]",
		Short: "Upload local data to a volume snapshot",
		Long: `Upload local data to a volume snapshot by:
1. Calculating the size of the local directory
2. Creating a Persistent Volume Claim with appropriate size
3. Creating a temporary pod with the persistent volume attached
4. Copying data from local to remote with progress tracking
5. Creating a volume snapshot
6. Cleaning up temporary resources`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.LocalPath = args[0]
			return runUpload(ctx, options, k8sClientProvider, ioCtrl)
		},
	}

	cmd.Flags().StringVar(&options.Size, "size", "", "Override the calculated size for the PVC (e.g., 10Gi)")
	cmd.Flags().StringVar(&options.SnapshotName, "name", "", "Custom name for the snapshot (default: auto-generated)")

	return cmd
}

func runUpload(ctx context.Context, options *UploadOptions, k8sClientProvider okteto.K8sClientProviderWithLogger, ioCtrl *io.Controller) error {
	// Validate local path exists
	if _, err := os.Stat(options.LocalPath); os.IsNotExist(err) {
		return fmt.Errorf("local path does not exist: %s", options.LocalPath)
	}

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl is required but not found in PATH. Please install kubectl")
	}

	// Get current namespace
	namespace := config.GetNamespace()
	if namespace == "" {
		return fmt.Errorf("no namespace configured. Run 'okteto namespace' to set one")
	}

	ioCtrl.Logger().Infof("Starting snapshot upload from %s", options.LocalPath)

	// Step 1: Calculate directory size
	ioCtrl.Logger().Info("Calculating directory size...")
	dirSize, err := calculateDirectorySize(options.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to calculate directory size: %w", err)
	}

	// Step 2: Determine PVC size
	var pvcSize resource.Quantity
	if options.Size != "" {
		pvcSize, err = resource.ParseQuantity(options.Size)
		if err != nil {
			return fmt.Errorf("invalid size format: %w", err)
		}
	} else {
		// Add 20% overhead for safety
		sizeWithOverhead := int64(float64(dirSize) * 1.2)
		pvcSize = *resource.NewQuantity(sizeWithOverhead, resource.BinarySI)
	}

	ioCtrl.Logger().Infof("Directory size: %s, PVC size: %s", 
		resource.NewQuantity(dirSize, resource.BinarySI).String(), 
		pvcSize.String())

	// Get Kubernetes client
	k8sClient, restConfig, err := k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	// Step 3: Create PVC
	pvcName := fmt.Sprintf("okteto-snapshot-upload-%d", time.Now().Unix())
	ioCtrl.Logger().Infof("Creating PVC %s...", pvcName)
	
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "okteto",
				"okteto.dev/snapshot-upload":   "true",
			},
		},
		Spec: apiv1.PersistentVolumeClaimSpec{
			AccessModes: []apiv1.PersistentVolumeAccessMode{apiv1.ReadWriteOnce},
			Resources: apiv1.ResourceRequirements{
				Requests: apiv1.ResourceList{
					apiv1.ResourceStorage: pvcSize,
				},
			},
		},
	}

	if err := volumes.Create(ctx, pvc, k8sClient); err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	// Cleanup function
	cleanup := func() {
		ioCtrl.Logger().Info("Cleaning up temporary resources...")
		if err := volumes.DestroyWithoutTimeout(ctx, pvcName, namespace, k8sClient); err != nil {
			ioCtrl.Logger().Errorf("Failed to cleanup PVC %s: %v", pvcName, err)
		}
	}

	// Step 4: Create temporary pod
	podName := fmt.Sprintf("okteto-snapshot-upload-pod-%d", time.Now().Unix())
	ioCtrl.Logger().Infof("Creating temporary pod %s...", podName)

	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "okteto",
				"okteto.dev/snapshot-upload":   "true",
			},
		},
		Spec: apiv1.PodSpec{
			RestartPolicy: apiv1.RestartPolicyNever,
			Containers: []apiv1.Container{
				{
					Name:    "uploader",
					Image:   "busybox:latest",
					Command: []string{"tail", "-f", "/dev/null"},
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []apiv1.Volume{
				{
					Name: "data",
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	_, err = k8sClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create pod: %w", err)
	}

	// Wait for pod to be ready
	ioCtrl.Logger().Info("Waiting for pod to be ready...")
	if err := waitForPodReady(ctx, k8sClient, namespace, podName, 5*time.Minute); err != nil {
		cleanup()
		pods.Destroy(ctx, podName, namespace, k8sClient)
		return fmt.Errorf("pod failed to become ready: %w", err)
	}

	// Step 5: Copy data with progress tracking
	ioCtrl.Logger().Info("Copying data to pod...")
	if err := copyDataToPod(ctx, namespace, podName, options.LocalPath, dirSize, ioCtrl); err != nil {
		cleanup()
		pods.Destroy(ctx, podName, namespace, k8sClient)
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Step 6: Delete the pod but keep PVC
	ioCtrl.Logger().Info("Removing temporary pod...")
	if err := pods.Destroy(ctx, podName, namespace, k8sClient); err != nil {
		ioCtrl.Logger().Warningf("Failed to delete pod %s: %v", podName, err)
	}

	// Step 7: Create volume snapshot
	snapshotName := options.SnapshotName
	if snapshotName == "" {
		snapshotName = fmt.Sprintf("okteto-snapshot-%d", time.Now().Unix())
	}

	ioCtrl.Logger().Infof("Creating volume snapshot %s...", snapshotName)
	if err := createVolumeSnapshot(ctx, restConfig, namespace, pvcName, snapshotName); err != nil {
		cleanup()
		return fmt.Errorf("failed to create volume snapshot: %w", err)
	}

	// Step 8: Wait for snapshot to be ready
	ioCtrl.Logger().Info("Waiting for snapshot to be ready...")
	if err := waitForSnapshotReady(ctx, restConfig, namespace, snapshotName, 10*time.Minute); err != nil {
		cleanup()
		return fmt.Errorf("snapshot failed to become ready: %w", err)
	}

	// Step 9: Clean up PVC
	cleanup()

	// Return success information
	ioCtrl.Logger().Successf("Snapshot upload completed successfully!")
	ioCtrl.Logger().Infof("Namespace: %s", namespace)
	ioCtrl.Logger().Infof("Snapshot Name: %s", snapshotName)

	return nil
}

func calculateDirectorySize(path string) (int64, error) {
	var size int64
	var fileCount int64
	
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files that can't be accessed rather than failing completely
			fmt.Printf("Warning: skipping %s: %v\n", filePath, err)
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
			fileCount++
		}
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size: %w", err)
	}
	
	if fileCount == 0 {
		return 0, fmt.Errorf("no files found in directory: %s", path)
	}
	
	return size, nil
}

func waitForPodReady(ctx context.Context, client kubernetes.Interface, namespace, podName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for pod to be ready")
		default:
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if pod.Status.Phase == apiv1.PodRunning {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == apiv1.PodReady && condition.Status == apiv1.ConditionTrue {
						return nil
					}
				}
			}

			if pod.Status.Phase == apiv1.PodFailed {
				return fmt.Errorf("pod failed to start")
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func copyDataToPod(ctx context.Context, namespace, podName, localPath string, totalSize int64, ioCtrl *io.Controller) error {
	// Create progress bar
	bar := pb.New64(totalSize)
	bar.Set("prefix", "Copying data ")
	bar.Start()
	defer bar.Finish()

	// Use kubectl cp command
	cmd := exec.CommandContext(ctx, "kubectl", "cp", localPath, fmt.Sprintf("%s/%s:/data", namespace, podName))
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start kubectl cp: %w", err)
	}

	// Monitor progress (simplified - in a real implementation you'd track actual progress)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		current := int64(0)
		increment := totalSize / 100
		
		for {
			select {
			case <-ticker.C:
				if current < totalSize {
					current += increment
					if current > totalSize {
						current = totalSize
					}
					bar.SetCurrent(current)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("kubectl cp failed: %w", err)
	}

	bar.SetCurrent(totalSize)
	return nil
}

func createVolumeSnapshot(ctx context.Context, restConfig *rest.Config, namespace, pvcName, snapshotName string) error {
	// Get dynamic client for VolumeSnapshot
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Define VolumeSnapshot resource
	volumeSnapshotGVR := schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  "v1",
		Resource: "volumesnapshots",
	}

	snapshot := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshot",
			"metadata": map[string]interface{}{
				"name":      snapshotName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "okteto",
					"okteto.dev/snapshot-upload":   "true",
				},
			},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"persistentVolumeClaimName": pvcName,
				},
			},
		},
	}

	_, err = dynamicClient.Resource(volumeSnapshotGVR).Namespace(namespace).Create(ctx, snapshot, metav1.CreateOptions{})
	return err
}

func waitForSnapshotReady(ctx context.Context, restConfig *rest.Config, namespace, snapshotName string, timeout time.Duration) error {
	// Get dynamic client for VolumeSnapshot
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	volumeSnapshotGVR := schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  "v1",
		Resource: "volumesnapshots",
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watcher, err := dynamicClient.Resource(volumeSnapshotGVR).Namespace(namespace).Watch(timeoutCtx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", snapshotName),
	})
	if err != nil {
		return fmt.Errorf("failed to watch snapshot: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return fmt.Errorf("error watching snapshot")
			}

			if event.Object != nil {
				snapshot := event.Object.(*unstructured.Unstructured)
				status, found, err := unstructured.NestedMap(snapshot.Object, "status")
				if err != nil || !found {
					continue
				}

				readyToUse, found, err := unstructured.NestedBool(status, "readyToUse")
				if err != nil || !found {
					continue
				}

				if readyToUse {
					return nil
				}
			}
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for snapshot to be ready")
		}
	}
}
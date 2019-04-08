package model

import (
	"fmt"
)

const (
	// CNDLabel is the label added to a dev deployment in k8
	CNDLabel = "cnd.okteto.com/deployment"

	// CNDDeploymentAnnotation is the original deployment manifest
	CNDDeploymentAnnotation = "cnd.okteto.com/deployment"

	// CNDAutoDestroyOnDown marks that the deployment has to be deleted on down
	CNDAutoDestroyOnDown = "cnd.okteto.com/auto-destroy"

	// CNDDevListAnnotation is the list of cnd manifest annotations
	CNDDevListAnnotation = "cnd.okteto.com/cnd"

	// CNDSyncContainer is the name of the container running syncthing
	CNDSyncContainer = "cnd-sync"

	// CNDSyncSecretVolume is the name of the volume mounting the secret
	CNDSyncSecretVolume = "cnd-sync-secret"

	cndDevAnnotationTemplate     = "cnd.okteto.com/dev-%s"
	cndInitSyncContainerTemplate = "cnd-init-%s"
	cndDinDContainerTemplate     = "dind-%s"
	cndSyncVolumeTemplate        = "cnd-sync-data-%s-%s"
	cndDinDVolumeTemplate        = "cnd-dind-storage-%s-%s"
	cndDataVolumeTemplate        = "cnd-data-%s-%s-%s"
	cndSyncMountTemplate         = "/var/cnd-sync/%s"
	cndSyncSecretTemplate        = "cnd-secret-%s"
)

// GetCNDInitSyncContainer returns the CND init sync container name for a given container
func (dev *Dev) GetCNDInitSyncContainer() string {
	return fmt.Sprintf(cndInitSyncContainerTemplate, dev.Container)
}

// GetCNDDinDContainer returns the CND dind container name for a given container
func (dev *Dev) GetCNDDinDContainer() string {
	return fmt.Sprintf(cndDinDContainerTemplate, dev.Container)
}

// GetCNDSyncVolume returns the CND sync volume name for a given container
func (dev *Dev) GetCNDSyncVolume() string {
	return fmt.Sprintf(cndSyncVolumeTemplate, dev.Name, dev.Container)
}

// GetCNDDinDVolume returns the CND dind volume name for a given container
func (dev *Dev) GetCNDDinDVolume() string {
	return fmt.Sprintf(cndDinDVolumeTemplate, dev.Name, dev.Container)
}

// GetCNDDataVolume returns the CND data volume name for a given container
func (dev *Dev) GetCNDDataVolume(v Volume) string {
	return fmt.Sprintf(cndDataVolumeTemplate, dev.Name, dev.Container, v.Name)
}

// GetCNDSyncMount returns the CND sync mount for a given container
func (dev *Dev) GetCNDSyncMount() string {
	return fmt.Sprintf(cndSyncMountTemplate, dev.Container)
}

// GetCNDSyncSecret returns the CND sync secret for a given deployment
func GetCNDSyncSecret(deployment string) string {
	return fmt.Sprintf(cndSyncSecretTemplate, deployment)
}

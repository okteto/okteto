package labels

const (
	//OktetoVersion represents the current dev data version
	OktetoVersion = "1.0"

	// OktetoDevLabel indicates the dev pod
	OktetoDevLabel = "dev.okteto.com"

	// OktetoInteractiveDevLabel indicates the interactive dev pod
	OktetoInteractiveDevLabel = "interactive.dev.okteto.com"

	// OktetoDetachedDevLabel indicates the detached dev pods
	OktetoDetachedDevLabel = "detached.dev.okteto.com"

	// OktetoTranslationAnnotation sets the translation rules
	OktetoTranslationAnnotation = "dev.okteto.com/translation"

	// OktetoSyncLabel indicates a synthing pod
	OktetoSyncLabel = "syncthing.okteto.com"
)

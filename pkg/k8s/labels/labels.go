package labels

const (
	//Version represents the current dev data version
	Version = "1.0"

	// DevLabel indicates the dev pod
	DevLabel = "dev.okteto.com"

	// InteractiveDevLabel indicates the interactive dev pod
	InteractiveDevLabel = "interactive.dev.okteto.com"

	// DetachedDevLabel indicates the detached dev pods
	DetachedDevLabel = "detached.dev.okteto.com"

	// TranslationAnnotation sets the translation rules
	TranslationAnnotation = "dev.okteto.com/translation"

	// SyncLabel indicates a synthing pod
	SyncLabel = "syncthing.okteto.com"
)

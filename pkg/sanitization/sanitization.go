package sanitization

import (
	"regexp"
	"strings"
)

var (
	// validKubeNameRegex is the regex to validate a kubernetes resource name
	validKubeNameRegex = regexp.MustCompile(`[^a-z0-9\-]+`)

	// moreThanOneHyphen is the regex to check if there is more than one hyphen together
	moreThanOneHyphen = regexp.MustCompile(`-(-+)`)
)

const (
	// maxK8sResourceMetaLength is the max length a string can have to be considered a kubernetes resource name, label, annotation, etc
	maxK8sResourceMetaLength = 63
)

func SanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)

	name = validKubeNameRegex.ReplaceAllString(name, "-")

	name = moreThanOneHyphen.ReplaceAllString(name, "-")
	// trim the repository name for internal use in labels
	if len(name) > maxK8sResourceMetaLength {
		name = name[:maxK8sResourceMetaLength]
	}
	name = strings.TrimSuffix(name, "-")
	name = strings.TrimPrefix(name, "-")
	return name
}

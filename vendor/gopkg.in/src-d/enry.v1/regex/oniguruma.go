// +build oniguruma

package regex

import (
	"github.com/moovweb/rubex"
)

type EnryRegexp = *rubex.Regexp

func MustCompile(str string) EnryRegexp {
	return rubex.MustCompile(str)
}

func QuoteMeta(s string) string {
	return rubex.QuoteMeta(s)
}

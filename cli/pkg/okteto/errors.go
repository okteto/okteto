package okteto

import "fmt"

var errNoLogin error = fmt.Errorf("please run 'okteto login'")

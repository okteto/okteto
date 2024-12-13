package ignore

type IgnoreFunc func(filePath string) (bool, error)

func (fn IgnoreFunc) Ignore(filePath string) (bool, error) {
	return fn(filePath)
}

// Never is a noop ingorer that never ignores a file
var Never = IgnoreFunc(func(filePath string) (bool, error) {
	return false, nil
})

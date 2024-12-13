package ignore

type Ignorer interface {
	Ignore(filePath string) (bool, error)
}

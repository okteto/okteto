package ignore

type MultiIgnorer struct {
	ignorers []Ignorer
}

func (mi *MultiIgnorer) Ignore(filePath string) (bool, error) {
	for _, i := range mi.ignorers {
		if i == nil {
			continue
		}
		ignore, err := i.Ignore(filePath)
		if err != nil {
			return false, err
		}
		if ignore {
			return true, nil
		}
	}
	return false, nil
}

func NewMultiIgnorer(ignorers ...Ignorer) *MultiIgnorer {
	return &MultiIgnorer{ignorers: ignorers}
}

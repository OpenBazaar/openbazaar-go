package rlepluslazy

// Union returns the union of the passed iterators. Internally, this calls Or on
// the passed iterators, combining them with a binary tree of Ors.
func Union(iters ...RunIterator) (RunIterator, error) {
	if len(iters) == 0 {
		return RunsFromSlice(nil)
	}

	for len(iters) > 1 {
		var next []RunIterator

		for i := 0; i < len(iters); i += 2 {
			if i+1 >= len(iters) {
				next = append(next, iters[i])
				continue
			}

			orit, err := Or(iters[i], iters[i+1])
			if err != nil {
				return nil, err
			}

			next = append(next, orit)
		}

		iters = next
	}

	return iters[0], nil
}

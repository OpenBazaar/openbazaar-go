package rlepluslazy

func JoinClose(it RunIterator, closeness uint64) (RunIterator, error) {
	jc := &jcIt{
		it:        &peekIter{it: it},
		closeness: closeness,
	}
	if err := jc.prep(); err != nil {
		return nil, err
	}
	return jc, nil
}

type jcIt struct {
	it  *peekIter
	run Run

	closeness uint64
}

func (jc *jcIt) prep() error {
	if !jc.it.HasNext() {
		jc.run = Run{}
		return nil
	}

	var err error
	jc.run, err = jc.it.NextRun()
	if err != nil {
		return err
	}

	if jc.run.Val {
		for {
			if jc.it.HasNext() {
				run, err := jc.it.NextRun()
				if err != nil {
					return err
				}
				if run.Len <= jc.closeness || run.Val {
					jc.run.Len += run.Len
					continue
				} else {
					jc.it.put(run, err)
					break
				}
			}
			break
		}
	}
	return nil
}

func (jc *jcIt) HasNext() bool {
	return jc.run.Valid()
}

func (jc *jcIt) NextRun() (Run, error) {
	out := jc.run
	if err := jc.prep(); err != nil {
		return Run{}, err
	}
	return out, nil

}

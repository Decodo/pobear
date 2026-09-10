package pool

import "sync"

type Pool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

func New(concurrency int) *Pool {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Pool{sem: make(chan struct{}, concurrency)}
}

func (p *Pool) Submit(fn func()) {
	p.wg.Go(func() {
		p.sem <- struct{}{}
		defer func() { <-p.sem }()
		fn()
	})
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

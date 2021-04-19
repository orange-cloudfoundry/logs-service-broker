package syslog

import (
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"
)

type MultiWriter struct {
	mw []io.WriteCloser
}

func MultiDial(addresses ...string) (*MultiWriter, error) {
	mw := make([]io.WriteCloser, len(addresses))
	for i, addr := range addresses {
		w, err := NewWriter(addr)
		if err != nil {
			return nil, err
		}
		mw[i] = w
	}
	return &MultiWriter{mw}, nil
}

func (t *MultiWriter) Write(b []byte) (int, error) {
	var wg sync.WaitGroup
	mutex := &sync.Mutex{}
	wg.Add(len(t.mw))
	var result error
	for _, w := range t.mw {
		go func(w io.WriteCloser) {
			defer wg.Done()
			_, err := w.Write(b)
			if err != nil {
				mutex.Lock()
				result = multierror.Append(result, err)
				mutex.Unlock()
			}
		}(w)
	}
	wg.Wait()
	return len(b), result
}

func (t *MultiWriter) Close() error {
	var result error

	for _, w := range t.mw {
		err := w.Close()
		if err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

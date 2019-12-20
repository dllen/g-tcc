package tcc

import (
	"errors"
	"sync"

	"github.com/cenkalti/backoff/v3"
	"github.com/rs/xid"
	"golang.org/x/sync/errgroup"
)

// Option can set option to service
// Option can be passed to NewService() and NewDirector,
// if you pass it to both, the one which is passed to NewDirector will be used
type Option func(s *director)

// WithMaxRetries sets limitation of retry times
func WithMaxRetries(maxRetries uint64) Option {
	return func(d *director) {
		d.backoff = backoff.WithMaxRetries(backoff.NewExponentialBackOff(), maxRetries)
	}
}

// Director can direct multiple service
// First, call every service's try() asynchronously.
// If all the try succeeded, call every service's confirm().
// If even one of the services' try fails, every service's cancel will be called.
type Director interface {
	Direct() error
}

type director struct {
	services []*Service
	backoff  backoff.BackOff

	sync.Mutex
}

// NewDirector returns interface Director
func NewDirector(services []*Service, opts ...Option) Director {
	maxRetries := uint64(10)
	txId := xid.New().String()
	for _, service := range services {
		service.txId = txId
		service.tried = false
		service.trySucceeded = false
		service.canceled = false
		service.cancelSucceeded = false
		service.confirmed = false
		service.cancelSucceeded = false
	}
	o := &director{
		services: services,
		backoff:  backoff.WithMaxRetries(backoff.NewExponentialBackOff(), maxRetries),
		Mutex:    sync.Mutex{},
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Direct can handle all the passed Service's transaction
func (d *director) Direct() error {
	if tryErr := d.tryAll(); tryErr != nil {
		if cancelErr := d.cancelAll(); cancelErr != nil {
			return cancelErr
		}
		return tryErr
	}
	return d.confirmAll()
}

func (d *director) tryAll() error {
	eg := errgroup.Group{}
	for _, s := range d.services {
		s := s
		eg.Go(func() error {
			s.tried = true
			err := s.Try()
			if err != nil {
				return &Error{
					failedPhase: ErrTryFailed,
					err:         err,
					serviceName: s.name,
				}
			}
			s.trySucceeded = true
			return nil
		})
	}
	return eg.Wait()
}

func (d *director) confirmAll() error {
	eg := errgroup.Group{}
	for _, s := range d.services {
		s := s
		eg.Go(func() error {
			s.confirmed = true
			if !s.trySucceeded {
				return &Error{
					failedPhase: ErrConfirmFailed,
					err:         errors.New("try did not succeed"),
					serviceName: s.name,
				}
			}
			d.Lock()
			defer d.Unlock()
			err := backoff.Retry(s.Confirm, d.backoff)
			if err != nil {
				return &Error{
					failedPhase: ErrConfirmFailed,
					err:         err,
					serviceName: s.name,
				}
			}
			s.confirmSucceeded = true
			return nil
		})
	}
	return eg.Wait()
}

func (d *director) cancelAll() error {
	eg := errgroup.Group{}
	for _, s := range d.services {
		s := s
		eg.Go(func() error {
			if !s.trySucceeded {
				return nil
			}
			s.canceled = true
			d.Lock()
			defer d.Unlock()
			err := backoff.Retry(s.Cancel, d.backoff)
			if err != nil {
				return &Error{
					failedPhase: ErrCancelFailed,
					err:         err,
					serviceName: s.name,
				}
			}
			s.cancelSucceeded = true
			return nil
		})
	}
	return eg.Wait()
}

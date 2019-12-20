package tcc

import (
	"errors"
	"testing"

	"github.com/cenkalti/backoff/v3"
)

func Test_director_Direct_No_Error(t *testing.T) {
	type fields struct {
		services []*Service
		backoff  backoff.BackOff
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "no error",
			fields: fields{
				services: []*Service{
					NewService(
						"s1",
						func() error { // try
							return nil
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return nil
						},
					),
					NewService(
						"s2",
						func() error { // try
							return nil
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return nil
						},
					),
				},
				backoff: backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 1),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &director{
				services: tt.fields.services,
				backoff:  tt.fields.backoff,
			}
			if err := o.Direct(); (err != nil) != tt.wantErr {
				t.Errorf("director.Direct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_director_Direct_Error(t *testing.T) {
	type fields struct {
		services []*Service
	}
	tests := []struct {
		name        string
		fields      fields
		wantErr     bool
		FailedPhase int
	}{
		{
			name: "try phase error, cancel will be called",
			fields: fields{
				services: []*Service{
					NewService(
						"s1",
						func() error { // try
							return nil
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return nil
						},
					),
					NewService(
						"s2",
						func() error { // try
							return errors.New("test")
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return nil
						},
					),
				},
			},
			wantErr:     true,
			FailedPhase: ErrTryFailed,
		},
		{
			name: "confirm phase error",
			fields: fields{
				services: []*Service{
					NewService(
						"s1",
						func() error { // try
							return nil
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return nil
						},
					),
					NewService(
						"s2",
						func() error { // try
							return nil
						},
						func() error { // confirm
							return errors.New("test")
						},
						func() error { // cancel
							return nil
						},
					),
				},
			},
			wantErr:     true,
			FailedPhase: ErrConfirmFailed,
		},
		{
			name: "cancel phase error",
			fields: fields{
				services: []*Service{
					NewService(
						"s1",
						func() error { // try
							return nil
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return nil
						},
					),
					NewService(
						"s2",
						func() error { // try
							return errors.New("test")
						},
						func() error { // confirm
							return nil
						},
						func() error { // cancel
							return errors.New("test")
						},
					),
				},
			},
			wantErr:     true,
			FailedPhase: ErrCancelFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewDirector(
				tt.fields.services,
				WithMaxRetries(1),
			)
			err := o.Direct()
			if (err != nil) != tt.wantErr {
				t.Errorf("director.Direct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			e, ok := err.(*Error)
			if !ok {
				t.Errorf("cannot cast to Error")
				return
			}
			if e.FailedPhase() != tt.FailedPhase {
				t.Errorf("cannot cast to Error")
				return
			}
		})
	}
}

package main

import (
	"errors"
)

const (
	outWidth   = 640
	outHeight  = 360
	outBitRate = 1_000_000
	defaultFPS = 25
)

type Options struct {
	Input    string
	Output   string
	Offset   float64
	Duration float64
}

func (o Options) Validate() error {
	if o.Input == "" {
		return errors.New("input path is required")
	}
	if o.Output == "" {
		return errors.New("output path is required")
	}
	if o.Duration <= 0 {
		return errors.New("duration must be positive")
	}
	return nil
}

func Transcode(opts Options) error {
	return nil
}

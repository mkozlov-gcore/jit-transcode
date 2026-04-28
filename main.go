package main

import (
	"flag"
	"log"
)

func main() {
	input := flag.String("input", "", "path to input video file")
	output := flag.String("output", "", "path to output .ts file")
	offset := flag.Float64("offset", 0, "start offset in seconds")
	duration := flag.Float64("duration", 0, "duration in seconds")
	flag.Parse()

	opts := Options{
		Input:    *input,
		Output:   *output,
		Offset:   *offset,
		Duration: *duration,
	}
	if err := opts.Validate(); err != nil {
		flag.Usage()
		log.Fatal(err)
	}

	if err := Transcode(opts); err != nil {
		log.Fatal(err)
	}
}

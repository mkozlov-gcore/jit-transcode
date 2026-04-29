package main

import (
	"errors"
	"fmt"
	"math"

	"github.com/asticode/go-astiav"
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

func ProbeDuration(input string) (float64, error) {
	fc := astiav.AllocFormatContext()
	if fc == nil {
		return 0, errors.New("failed to alloc format context")
	}
	defer fc.Free()

	if err := fc.OpenInput(input, nil, nil); err != nil {
		return 0, fmt.Errorf("opening input: %w", err)
	}
	defer fc.CloseInput()

	if err := fc.FindStreamInfo(nil); err != nil {
		return 0, fmt.Errorf("finding stream info: %w", err)
	}

	dur := fc.Duration()
	if dur <= 0 {
		return 0, errors.New("could not determine video duration")
	}
	return float64(dur) / float64(astiav.TimeBase), nil
}

func Transcode(opts Options) error {
	astiav.SetLogLevel(astiav.LogLevelError)

	// --- Input context ---
	inputFC := astiav.AllocFormatContext()
	if inputFC == nil {
		return errors.New("failed to alloc input format context")
	}
	defer inputFC.Free()

	if err := inputFC.OpenInput(opts.Input, nil, nil); err != nil {
		return fmt.Errorf("opening input %q: %w", opts.Input, err)
	}
	defer inputFC.CloseInput()

	if err := inputFC.FindStreamInfo(nil); err != nil {
		return fmt.Errorf("finding stream info: %w", err)
	}

	// --- Video stream ---
	var inputStream *astiav.Stream
	for _, s := range inputFC.Streams() {
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			inputStream = s
			break
		}
	}
	if inputStream == nil {
		return errors.New("no video stream found in input")
	}

	// --- Decoder ---
	decoder := astiav.FindDecoder(inputStream.CodecParameters().CodecID())
	if decoder == nil {
		return fmt.Errorf("decoder not found for codec %s", inputStream.CodecParameters().CodecID())
	}

	decoderCtx := astiav.AllocCodecContext(decoder)
	if decoderCtx == nil {
		return errors.New("failed to alloc decoder context")
	}
	defer decoderCtx.Free()

	if err := inputStream.CodecParameters().ToCodecContext(decoderCtx); err != nil {
		return fmt.Errorf("copying codec parameters to decoder: %w", err)
	}

	if err := decoderCtx.Open(decoder, nil); err != nil {
		return fmt.Errorf("opening decoder: %w", err)
	}

	// --- FPS ---
	fpsRat := inputStream.AvgFrameRate()
	fpsFloat := float64(fpsRat.Num()) / float64(fpsRat.Den())
	if fpsFloat <= 0 {
		fpsFloat = defaultFPS
	}

	// --- Seek ---
	seekTS := int64(opts.Offset * float64(astiav.TimeBase))
	if err := inputFC.SeekFrame(-1, seekTS, astiav.NewSeekFlags(astiav.SeekFlagBackward)); err != nil {
		return fmt.Errorf("seeking to offset %.2fs: %w", opts.Offset, err)
	}
	// No decoder flush needed: seek happens before any packets are sent to the decoder.

	// --- Scaler ---
	swsCtx, err := astiav.CreateSoftwareScaleContext(
		decoderCtx.Width(), decoderCtx.Height(), decoderCtx.PixelFormat(),
		outWidth, outHeight, astiav.PixelFormatYuv420P,
		astiav.NewSoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagBilinear),
	)
	if err != nil {
		return fmt.Errorf("creating swscale context: %w", err)
	}
	defer swsCtx.Free()

	scaledFrame := astiav.AllocFrame()
	if scaledFrame == nil {
		return errors.New("failed to alloc scaled frame")
	}
	defer scaledFrame.Free()

	scaledFrame.SetWidth(outWidth)
	scaledFrame.SetHeight(outHeight)
	scaledFrame.SetPixelFormat(astiav.PixelFormatYuv420P)
	if err := scaledFrame.AllocBuffer(0); err != nil {
		return fmt.Errorf("allocating scaled frame buffer: %w", err)
	}

	// --- Encoder ---
	encoder := astiav.FindEncoderByName("libx264")
	if encoder == nil {
		return errors.New("libx264 encoder not found (install libx264 and rebuild ffmpeg with --enable-libx264)")
	}

	encoderCtx := astiav.AllocCodecContext(encoder)
	if encoderCtx == nil {
		return errors.New("failed to alloc encoder context")
	}
	defer encoderCtx.Free()

	encoderCtx.SetWidth(outWidth)
	encoderCtx.SetHeight(outHeight)
	encoderCtx.SetPixelFormat(astiav.PixelFormatYuv420P)
	encoderCtx.SetBitRate(outBitRate)

	fpsInt := int(math.Round(fpsFloat))
	if fpsInt <= 0 {
		fpsInt = defaultFPS
	}
	encoderCtx.SetTimeBase(astiav.NewRational(1, fpsInt))
	encoderCtx.SetGopSize(int(math.Round(fpsFloat * 2)))
	encoderCtx.SetMaxBFrames(0)

	// x264 options via dictionary
	encDict := astiav.NewDictionary()
	defer encDict.Free()
	if err := encDict.Set("preset", "fast", 0); err != nil {
		return fmt.Errorf("setting encoder dictionary: %w", err)
	}

	if err := encoderCtx.Open(encoder, encDict); err != nil {
		return fmt.Errorf("opening encoder: %w", err)
	}

	// --- Output muxer ---
	outputFC, err := astiav.AllocOutputFormatContext(nil, "mpegts", opts.Output)
	if err != nil {
		return fmt.Errorf("allocating output context: %w", err)
	}
	defer outputFC.Free()

	outputStream := outputFC.NewStream(nil)
	if outputStream == nil {
		return errors.New("failed to create output stream")
	}

	if err := encoderCtx.ToCodecParameters(outputStream.CodecParameters()); err != nil {
		return fmt.Errorf("copying encoder parameters to output stream: %w", err)
	}
	outputStream.SetTimeBase(encoderCtx.TimeBase())

	// Open output file IO
	if !outputFC.OutputFormat().Flags().Has(astiav.IOFormatFlagNofile) {
		pb, err := astiav.OpenIOContext(opts.Output, astiav.NewIOContextFlags(astiav.IOContextFlagWrite), nil, nil)
		if err != nil {
			return fmt.Errorf("opening output file %q: %w", opts.Output, err)
		}
		defer pb.Close()
		outputFC.SetPb(pb)
	}

	if err := outputFC.WriteHeader(nil); err != nil {
		return fmt.Errorf("writing MPEG-TS header: %w", err)
	}

	// --- Main loop ---
	pkt := astiav.AllocPacket()
	if pkt == nil {
		return errors.New("failed to alloc packet")
	}
	defer pkt.Free()

	decodedFrame := astiav.AllocFrame()
	if decodedFrame == nil {
		return errors.New("failed to alloc decoded frame")
	}
	defer decodedFrame.Free()

	encodedPkt := astiav.AllocPacket()
	if encodedPkt == nil {
		return errors.New("failed to alloc encoded packet")
	}
	defer encodedPkt.Free()

	endSecs := opts.Offset + opts.Duration
	// Start PTS at offset so timestamps in the output segment are continuous
	// relative to the source file (offset * fps frames).
	outPTS := int64(math.Round(opts.Offset * fpsFloat))

	writeEncodedPackets := func() error {
		for {
			if err := encoderCtx.ReceivePacket(encodedPkt); err != nil {
				if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
					break
				}
				return fmt.Errorf("receiving encoded packet: %w", err)
			}
			encodedPkt.SetStreamIndex(outputStream.Index())
			encodedPkt.RescaleTs(encoderCtx.TimeBase(), outputStream.TimeBase())
			if err := outputFC.WriteInterleavedFrame(encodedPkt); err != nil {
				return fmt.Errorf("writing packet to muxer: %w", err)
			}
			encodedPkt.Unref()
		}
		return nil
	}

	done := false
	for !done {
		if err := inputFC.ReadFrame(pkt); err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			return fmt.Errorf("reading frame: %w", err)
		}

		if pkt.StreamIndex() != inputStream.Index() {
			pkt.Unref()
			continue
		}

		if err := decoderCtx.SendPacket(pkt); err != nil {
			pkt.Unref()
			return fmt.Errorf("sending packet to decoder: %w", err)
		}
		pkt.Unref()

		for {
			if err := decoderCtx.ReceiveFrame(decodedFrame); err != nil {
				if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
					break
				}
				return fmt.Errorf("receiving decoded frame: %w", err)
			}

			tb := inputStream.TimeBase()
			frameSecs := float64(decodedFrame.Pts()) * float64(tb.Num()) / float64(tb.Den())

			if frameSecs < opts.Offset {
				decodedFrame.Unref()
				continue
			}

			if frameSecs >= endSecs {
				done = true
				decodedFrame.Unref()
				break
			}

			if err := swsCtx.ScaleFrame(decodedFrame, scaledFrame); err != nil {
				decodedFrame.Unref()
				return fmt.Errorf("scaling frame: %w", err)
			}
			decodedFrame.Unref()

			scaledFrame.SetPts(outPTS)
			outPTS++

			if err := encoderCtx.SendFrame(scaledFrame); err != nil {
				return fmt.Errorf("sending frame to encoder: %w", err)
			}

			if err := writeEncodedPackets(); err != nil {
				return err
			}
		}
	}

	// Flush encoder
	if err := encoderCtx.SendFrame(nil); err != nil {
		return fmt.Errorf("flushing encoder: %w", err)
	}
	if err := writeEncodedPackets(); err != nil {
		return err
	}

	if err := outputFC.WriteTrailer(); err != nil {
		return fmt.Errorf("writing trailer: %w", err)
	}

	return nil
}

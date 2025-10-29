package kiss

import (
	"bufio"
	"bytes"
	"io"
)

// KISS protocol constants
const (
	FEND byte = 0xC0 // Frame End
	FESC byte = 0xDB // Frame Escape
	TFEND byte = 0xDC // Transposed Frame End
	TFESC byte = 0xDD // Transposed Frame Escape
)

// Decoder reads KISS frames from an io.Reader
type Decoder struct {
	r *bufio.Reader
}

// NewDecoder creates a new KISS frame decoder
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

// ReadFrame reads a single, complete KISS frame
// It handles FEND delimiters and FESC escaping
func (d *Decoder) ReadFrame() ([]byte, error) {
	var frame bytes.Buffer
	inFrame := false

	for {
		b, err := d.r.ReadByte()
		if err != nil {
			return nil, err // Error (e.g., EOF)
		}

		switch b {
		case FEND:
			if inFrame {
				// End of frame
				if frame.Len() > 0 {
					return frame.Bytes(), nil
				}
				// else, empty frame (e.g., FEND FEND), keep waiting
			} else {
				// Start of frame
				inFrame = true
			}
		case FESC:
			if !inFrame {
				continue // Not in a frame, ignore
			}
			// Read next byte
			b, err = d.r.ReadByte()
			if err != nil {
				return nil, err
			}
			// Un-escape
			switch b {
			case TFEND:
				frame.WriteByte(FEND)
			case TFESC:
				frame.WriteByte(FESC)
			default:
				// Protocol error, but we'll be lenient
				frame.WriteByte(b)
			}
		default:
			if inFrame {
				frame.WriteByte(b)
			}
		}
	}
}
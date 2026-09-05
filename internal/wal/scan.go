package wal

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// walkRecords reads a WAL file from offset 0.
//
// validEnd is the byte offset after the last well-formed record (at least
// headerSize when the header is intact). An incomplete trailing record stops
// the walk and returns that validEnd with a nil error (torn tail).
//
// A full frame with a bad CRC and no further bytes is treated as a torn tail.
// A bad CRC with more bytes after it is mid-file corruption (ErrCorrupt).
func walkRecords(f *os.File, size int64, callback func(*Entry) error) (validEnd int64, err error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek WAL start: %w", err)
	}

	if size == 0 {
		return 0, nil
	}

	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(f, hdr); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return 0, nil // torn header
		}
		return 0, fmt.Errorf("failed to read WAL header: %w", err)
	}
	if err := checkHeader(hdr); err != nil {
		return 0, err
	}
	validEnd = int64(headerSize)

	lenBuf := make([]byte, 4)
	for {
		cur, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		if cur >= size {
			return validEnd, nil
		}

		if _, err := io.ReadFull(f, lenBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return validEnd, nil // torn length prefix
			}
			return 0, fmt.Errorf("failed to read record length: %w", err)
		}

		payloadLen := binary.BigEndian.Uint32(lenBuf)
		if payloadLen > 64<<20 {
			return 0, fmt.Errorf("%w: payload too large", ErrCorrupt)
		}

		posAfterLen, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		need := int64(payloadLen) + 4 // payload + crc
		if size-posAfterLen < need {
			return validEnd, nil // torn payload or crc
		}

		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(f, payload); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return validEnd, nil
			}
			return 0, fmt.Errorf("failed to read payload: %w", err)
		}
		crcBuf := make([]byte, 4)
		if _, err := io.ReadFull(f, crcBuf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return validEnd, nil
			}
			return 0, fmt.Errorf("failed to read crc: %w", err)
		}

		want := binary.BigEndian.Uint32(crcBuf)
		got := crc32.ChecksumIEEE(payload)
		posAfter, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		if want != got {
			if posAfter >= size {
				// Complete frame at EOF with bad CRC → treat as torn tail.
				return validEnd, nil
			}
			return 0, fmt.Errorf("%w: crc mismatch", ErrCorrupt)
		}

		entry, err := decodePayload(payload)
		if err != nil {
			return 0, err
		}
		if callback != nil {
			if err := callback(entry); err != nil {
				return 0, fmt.Errorf("replay callback failed: %w", err)
			}
		}
		validEnd = posAfter
	}
}

// repairTornTail truncates any incomplete trailing bytes so new appends land
// after the last good record (not past garbage that would hide them from Replay).
func repairTornTail(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat WAL: %w", err)
	}
	size := info.Size()

	validEnd, err := walkRecords(file, size, nil)
	if err != nil {
		return err
	}

	if validEnd == 0 {
		// Missing or torn header — rewrite a clean file.
		if err := file.Truncate(0); err != nil {
			return fmt.Errorf("failed to truncate torn WAL: %w", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek WAL: %w", err)
		}
		if _, err := file.Write(encodeHeader()); err != nil {
			return fmt.Errorf("failed to write WAL header: %w", err)
		}
		return file.Sync()
	}

	if validEnd < size {
		if err := file.Truncate(validEnd); err != nil {
			return fmt.Errorf("failed to truncate torn tail: %w", err)
		}
		if err := file.Sync(); err != nil {
			return fmt.Errorf("failed to sync after repair: %w", err)
		}
	}

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek WAL end: %w", err)
	}
	return nil
}

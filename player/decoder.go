package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
)

type Decoder func(path string) (beep.StreamSeekCloser, beep.Format, error)

var registry = map[string]Decoder{
	".mp3":  decodeMP3,
	".wav":  decodeWAV,
	".flac": decodeFLAC,
}

func Open(path string) (beep.StreamSeekCloser, beep.Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	dec, ok := registry[ext]
	if !ok {
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format: %s", ext)
	}
	return dec(path)
}

func decodeMP3(path string) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	s, format, err := mp3.Decode(f)
	if err != nil {
		_ = f.Close()
		return nil, beep.Format{}, err
	}
	// mp3.Decode 已经把文件句柄包进 streamer 里，关闭 streamer 会关文件
	return s, format, nil
}

func decodeWAV(path string) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	s, format, err := wav.Decode(f)
	if err != nil {
		_ = f.Close()
		return nil, beep.Format{}, err
	}
	return s, format, nil
}

func decodeFLAC(path string) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	s, format, err := flac.Decode(f)
	if err != nil {
		_ = f.Close()
		return nil, beep.Format{}, err
	}
	return s, format, nil
}

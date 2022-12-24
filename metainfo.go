package bytedribble

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/bunsenmcdubbs/bytedribble/bencoding"
	"io"
	"net/url"
)

// Metainfo describes a torrent.
//
// See: https://www.bittorrent.org/beps/bep_0003.html#metainfo-files
type Metainfo struct {
	TrackerURL     *url.URL          // announce
	Name           string            // info.name
	Hashes         [][sha1.Size]byte // info.pieces
	PieceSizeBytes int               // info.pieces length
	TotalSizeBytes int               // info.length
	Files          []struct {        // info.files
		SizeBytes int      // length
		Path      []string // path
	}
	RawInfo map[string]any // original map of entire "info" field
}

// ParseMetainfo parses a bencoded metainfo file
func ParseMetainfo(raw io.Reader) (Metainfo, error) {
	dict, err := bencoding.UnmarshalDict(bufio.NewReader(raw))
	if err != nil {
		return Metainfo{}, fmt.Errorf("bencoding: %w", err)
	}

	var meta Metainfo
	rawURL, ok := dict["announce"].(string)
	if !ok {
		return Metainfo{}, errors.New("missing announce url")
	}
	meta.TrackerURL, err = url.Parse(rawURL)
	if err != nil {
		return Metainfo{}, err
	}

	info, ok := dict["info"].(map[string]any)
	if !ok || info == nil {
		return Metainfo{}, errors.New("missing info")
	}
	meta.RawInfo = info

	meta.Name, ok = info["name"].(string)
	if !ok {
		return Metainfo{}, errors.New("missing name")
	}

	meta.PieceSizeBytes, ok = info["piece length"].(int)
	if !ok {
		return Metainfo{}, errors.New("missing number of pieces")
	}

	var numPieces int
	if length, ok := info["length"].(int); ok {
		meta.TotalSizeBytes = length
		numPieces = (meta.TotalSizeBytes + meta.PieceSizeBytes - 1) / meta.PieceSizeBytes
	}

	if files, ok := info["files"].([]any); ok {
		if meta.TotalSizeBytes != 0 {
			return Metainfo{}, errors.New("cannot specify total size with multiple files (length and files)")
		}

		for _, file := range files {
			parsedFile := struct { // info.files
				SizeBytes int      // length
				Path      []string // path
			}{}
			fileDict, ok := file.(map[string]any)
			if !ok {
				return Metainfo{}, errors.New("malformed metadata for child file")
			}
			parsedFile.SizeBytes, ok = fileDict["length"].(int)
			if !ok {
				return Metainfo{}, errors.New("missing child file length")
			}
			path, ok := fileDict["path"].([]any)
			if !ok {
				return Metainfo{}, errors.New("missing child file path")
			}
			for _, name := range path {
				nameString, ok := name.(string)
				if !ok {
					return Metainfo{}, errors.New("child path malformed")
				}
				parsedFile.Path = append(parsedFile.Path, nameString)
			}
			meta.Files = append(meta.Files, parsedFile)
			meta.TotalSizeBytes += parsedFile.SizeBytes
		}
		numPieces = len(meta.Files)
	}

	if meta.TotalSizeBytes == 0 {
		return Metainfo{}, errors.New("must specify either file size or include child file metadata")
	}

	hashes, ok := info["pieces"].(string)
	if !ok {
		return Metainfo{}, errors.New("missing piece hashes")
	}
	if len(hashes) != numPieces*sha1.Size {
		return Metainfo{}, errors.New("piece hashes are invalid")
	}
	meta.Hashes = make([][sha1.Size]byte, numPieces, numPieces)
	for i := range meta.Hashes {
		meta.Hashes[i] = *(*[sha1.Size]byte)([]byte(hashes)[i*sha1.Size : (i+1)*sha1.Size])
	}

	return meta, nil
}

func (m Metainfo) InfoHash() []byte {
	hash := sha1.Sum(bencoding.MarshalDict(m.RawInfo))
	return hash[:]
}

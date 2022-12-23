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

type Metainfo struct {
	URL            *url.URL          // announce
	Name           string            // info.name
	Hashes         [][sha1.Size]byte // info.pieces
	PieceSizeBytes int               //info.pieces length
	TotalSizeBytes int               // info.length
	Files          []struct {        // info.files
		SizeBytes int      // length
		Path      []string // path
	}
}

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
	meta.URL, err = url.Parse(rawURL)
	if err != nil {
		return Metainfo{}, err
	}

	info, ok := dict["info"].(map[string]any)
	if !ok || info == nil {
		return Metainfo{}, errors.New("missing info")
	}

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
	} else if files, ok := info["files"].([]any); ok {
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
		}
		numPieces = len(meta.Files)
	}
	if meta.TotalSizeBytes == 0 && len(meta.Files) == 0 {
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

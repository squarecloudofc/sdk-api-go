package squarecloud

import "encoding/json"

// FileType of a listed file manager entry.
type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
)

// FileInfo is one entry of GET /apps/{id}/files.
type FileInfo struct {
	Type FileType `json:"type"`
	Name string   `json:"name"`
	// Size in bytes. 0 for directories.
	Size int `json:"size"`
	// LastModified is a Unix timestamp in milliseconds.
	LastModified int64 `json:"lastModified"`
}

// FileContent is the response of GET /apps/{id}/files/content. The API
// returns a Node-style Buffer object ({"type":"Buffer","data":[...]}).
type FileContent struct {
	Type string    `json:"type"`
	Data ByteArray `json:"data"`
}

// ByteArray decodes a JSON array of numbers (Node Buffer data) into raw bytes.
type ByteArray []byte

func (b *ByteArray) UnmarshalJSON(data []byte) error {
	var ints []int
	if err := json.Unmarshal(data, &ints); err != nil {
		return err
	}

	out := make([]byte, len(ints))
	for i, v := range ints {
		out[i] = byte(v)
	}
	*b = out

	return nil
}

// FileWritten is the response of PUT /apps/{id}/files.
type FileWritten struct {
	Written bool `json:"written"`
}

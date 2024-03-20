package collect

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
)

type hasher struct {
	hash.Hash
}

func (h hasher) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(h.Sum(nil)))
}

type IntegrityCounter struct {
	FileCount uint   `json:"fileCount"`
	Hasher    hasher `json:"checksum"`
}

func (ic *IntegrityCounter) writeFile(data []byte) error {
	if ic.FileCount == 0 {
		ic.Hasher = hasher{sha512.New()}
	}
	ic.FileCount++

	_, err := ic.Hasher.Write(data)
	if err != nil {
		return fmt.Errorf("can't write data to hasher: %w", err)
	}

	return nil
}

type Counters struct {
	Dirs   map[string]IntegrityCounter `json:"directories"`
	Hasher hasher                      `json:"checksum"`
}

func NewCounters() *Counters {
	return &Counters{
		Dirs:   map[string]IntegrityCounter{},
		Hasher: hasher{sha512.New()},
	}
}

func (c *Counters) writeFile(dirPath string, data []byte) error {
	_, err := c.Hasher.Write(data)
	if err != nil {
		return fmt.Errorf("can't hash data: %w", err)
	}

	ic, found := c.Dirs[dirPath]
	if !found {
		ic = IntegrityCounter{}
	}

	err = ic.writeFile(data)
	if err != nil {
		return fmt.Errorf("can't write file to identity counter: %w", err)
	}

	c.Dirs[dirPath] = ic

	return nil
}

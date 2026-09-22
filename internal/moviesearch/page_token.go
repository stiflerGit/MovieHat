package moviesearch

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// TODO: generalize pageToken as we will use it in other endpoints
type pageToken struct {
	Value      string `json:"value"`
	NextOffset int    `json:"offset"`
}

func encodePageToken(v string, nextOffset int) (string, error) {
	b, err := json.Marshal(pageToken{Value: v, NextOffset: nextOffset})
	if err != nil {
		return "", fmt.Errorf("json.Marshal(pageToken): %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodePageToken(s string) (pageToken, error) {
	d, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return pageToken{}, fmt.Errorf("base64.RawURLEncoding.DecodeString: %w", err)
	}

	var t pageToken
	if err = json.Unmarshal(d, &t); err != nil {
		return pageToken{}, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return t, nil
}

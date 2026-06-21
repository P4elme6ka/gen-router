package meta

import (
	"fmt"
	"strconv"
	"strings"
)

type Tag struct {
	Raw string
	KV  map[string]string
}

func ParseTag(raw string) (Tag, error) {
	tag := Tag{Raw: raw, KV: map[string]string{}}
	if raw == "" {
		return tag, nil
	}

	parts := strings.Split(raw, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, ":")
		if !found {
			return Tag{}, fmt.Errorf("invalid gen-router tag part %q", part)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return Tag{}, fmt.Errorf("invalid gen-router tag part %q", part)
		}
		if _, exists := tag.KV[key]; exists {
			return Tag{}, fmt.Errorf("duplicate gen-router tag key %q", key)
		}
		tag.KV[key] = value
	}

	return tag, nil
}

func (t Tag) Value(key string) string {
	return t.KV[key]
}

func (t Tag) Source() string {
	return t.Value("in")
}

func (t Tag) Name() string {
	return t.Value("name")
}

func (t Tag) ResponseCode() (int, bool, error) {
	raw := t.Value("response")
	if raw == "" {
		return 0, false, nil
	}
	code, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("invalid response code %q", raw)
	}
	return code, true, nil
}

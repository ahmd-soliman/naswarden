package truenas

import (
	"encoding/json"
	"time"
)

// parseTrueNASTime parses TrueNAS middleware datetime representations into
// standard Unix seconds. TrueNAS DDP responses typically encode timestamps as
// {"$date": 1789769434000} (milliseconds), but can also return raw integers
// or RFC3339 strings.
func parseTrueNASTime(raw json.RawMessage) *int64 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var wrapper struct {
		Date int64 `json:"$date"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Date != 0 {
		sec := wrapper.Date / 1000
		return &sec
	}

	var num int64
	if err := json.Unmarshal(raw, &num); err == nil && num != 0 {
		if num > 1e11 {
			num = num / 1000
		}
		return &num
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil && str != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, str); err == nil {
				sec := t.Unix()
				return &sec
			}
		}
	}

	return nil
}

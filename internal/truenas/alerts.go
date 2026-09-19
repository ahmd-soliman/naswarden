package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Alert represents an active or past notification from TrueNAS middleware's
// `alert.list` method.
type Alert struct {
	ID        string `json:"id"`
	UUID      string `json:"uuid"`
	Level     string `json:"level"`     // CRITICAL, ERROR, WARNING, INFO
	Source    string `json:"source"`    // e.g. Quota, Replication, System, Network
	Klass     string `json:"klass"`     // e.g. QuotaWarning, ReplicationSuccess
	Formatted string `json:"formatted"` // e.g. "Quota exceeded on dataset backup/tank/main. Used 82.62% (3.72 TiB of 4.5 TiB)."
	Text      string `json:"text"`
	Datetime  int64  `json:"datetime"` // Unix seconds
	Dismissed bool   `json:"dismissed"`
}

type rawAlert struct {
	ID        string          `json:"id"`
	UUID      string          `json:"uuid"`
	Level     string          `json:"level"`
	Source    string          `json:"source"`
	Klass     string          `json:"klass"`
	Formatted string          `json:"formatted"`
	Text      string          `json:"text"`
	Dismissed bool            `json:"dismissed"`
	Datetime  json.RawMessage `json:"datetime"`
}

// ListAlerts fetches un-dismissed alerts from TrueNAS via alert.list.
func ListAlerts(ctx context.Context, c *Client) ([]Alert, error) {
	raw, err := c.Call(ctx, "alert.list", []any{})
	if err != nil {
		return nil, fmt.Errorf("alert.list: %w", err)
	}

	var rawList []rawAlert
	if err := json.Unmarshal(raw, &rawList); err != nil {
		return nil, fmt.Errorf("alert.list: decode: %w", err)
	}

	alerts := make([]Alert, 0, len(rawList))
	for _, ra := range rawList {
		if ra.Dismissed {
			continue
		}
		var dt int64
		if ts := parseTrueNASTime(ra.Datetime); ts != nil {
			dt = *ts
		}
		formatted := ra.Formatted
		if formatted == "" {
			formatted = ra.Text
		}
		id := ra.ID
		if id == "" {
			id = ra.UUID
		}
		alerts = append(alerts, Alert{
			ID:        id,
			UUID:      ra.UUID,
			Level:     strings.ToUpper(ra.Level),
			Source:    ra.Source,
			Klass:     ra.Klass,
			Formatted: formatted,
			Text:      ra.Text,
			Datetime:  dt,
			Dismissed: ra.Dismissed,
		})
	}

	// Sort severity: CRITICAL/ERROR (0), WARNING (1), INFO (2), others (3)
	// Within same severity, newest first
	sort.Slice(alerts, func(i, j int) bool {
		pi := alertPriority(alerts[i].Level)
		pj := alertPriority(alerts[j].Level)
		if pi != pj {
			return pi < pj
		}
		return alerts[i].Datetime > alerts[j].Datetime
	})

	return alerts, nil
}

func alertPriority(level string) int {
	switch level {
	case "CRITICAL", "ERROR":
		return 0
	case "WARNING":
		return 1
	case "INFO":
		return 2
	default:
		return 3
	}
}

package truenas

import (
	"context"
	"encoding/json"
	"fmt"
)

// property is the wrapper shape TrueNAS's middleware uses for most
// dataset attributes -- confirmed directly against a live TrueNAS SCALE
// 25.10.7 box via `midclt call pool.dataset.query` (they are NOT plain
// values, despite the field names suggesting otherwise). Parsed is nil
// when the property is unset (rawvalue "0"); non-nil with the byte count
// when it's actually configured.
type property struct {
	Parsed *int64 `json:"parsed"`
}

type rawDataset struct {
	Name     string   `json:"name"`
	Used     property `json:"used"`
	Quota    property `json:"quota"`
	RefQuota property `json:"refquota"`
}

// Dataset is a dataset that has an actual quota configured -- datasets
// with neither quota nor refquota set are filtered out entirely (nothing
// to warn about). TrueNAS treats quota (counts snapshots) and refquota
// (doesn't) as independent, mutually-optional properties -- confirmed
// tonight that some datasets use one, some the other, never assume
// either is the one that's set.
type Dataset struct {
	Name        string `json:"name"`
	Used        int64  `json:"used"`
	Quota       int64  `json:"quota"`
	QuotaSource string `json:"quota_source"` // "quota" or "refquota"
}

// ListDatasets calls pool.dataset.query and returns only datasets with a
// quota or refquota actually configured.
func ListDatasets(ctx context.Context, c *Client) ([]Dataset, error) {
	raw, err := c.Call(ctx, "pool.dataset.query", []any{})
	if err != nil {
		return nil, fmt.Errorf("pool.dataset.query: %w", err)
	}
	var rawDatasets []rawDataset
	if err := json.Unmarshal(raw, &rawDatasets); err != nil {
		return nil, fmt.Errorf("pool.dataset.query: decode: %w", err)
	}

	var datasets []Dataset
	for _, rd := range rawDatasets {
		var quota int64
		var source string
		switch {
		case rd.Quota.Parsed != nil && *rd.Quota.Parsed > 0:
			quota = *rd.Quota.Parsed
			source = "quota"
		case rd.RefQuota.Parsed != nil && *rd.RefQuota.Parsed > 0:
			quota = *rd.RefQuota.Parsed
			source = "refquota"
		default:
			continue // no quota configured, nothing to report
		}

		var used int64
		if rd.Used.Parsed != nil {
			used = *rd.Used.Parsed
		}

		datasets = append(datasets, Dataset{
			Name:        rd.Name,
			Used:        used,
			Quota:       quota,
			QuotaSource: source,
		})
	}
	return datasets, nil
}

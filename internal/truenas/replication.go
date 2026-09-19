package truenas

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ReplicationTask represents a configured ZFS replication task from TrueNAS
// middleware's `replication.query` method.
type ReplicationTask struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Direction           string   `json:"direction"`
	Transport           string   `json:"transport"`
	SourceDatasets      []string `json:"source_datasets"`
	TargetDataset       string   `json:"target_dataset"`
	TargetPool          string   `json:"target_pool"`
	SourcePools         []string `json:"source_pools"`
	State               string   `json:"state"`
	JobState            string   `json:"job_state,omitempty"`
	ProgressPercent     *float64 `json:"progress_percent,omitempty"`
	ProgressDescription string   `json:"progress_description,omitempty"`
	LastSnapshot        string   `json:"last_snapshot,omitempty"`
	TimeStarted         *int64   `json:"time_started,omitempty"`
	TimeFinished        *int64   `json:"time_finished,omitempty"`
	DurationSeconds     *int64   `json:"duration_seconds,omitempty"`
	Enabled             bool     `json:"enabled"`
}

type rawReplication struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Direction      string   `json:"direction"`
	Transport      string   `json:"transport"`
	SourceDatasets []string `json:"source_datasets"`
	TargetDataset  string   `json:"target_dataset"`
	Auto           bool     `json:"auto"`
	Enabled        bool     `json:"enabled"`
	State          struct {
		State        string          `json:"state"`
		Datetime     json.RawMessage `json:"datetime"`
		LastSnapshot string          `json:"last_snapshot"`
		Warnings     []any           `json:"warnings"`
	} `json:"state"`
	Job *struct {
		ID           int64           `json:"id"`
		State        string          `json:"state"`
		TimeStarted  json.RawMessage `json:"time_started"`
		TimeFinished json.RawMessage `json:"time_finished"`
		Progress     *struct {
			Percent     *float64 `json:"percent"`
			Description string   `json:"description"`
		} `json:"progress"`
	} `json:"job"`
}

// ListReplications queries TrueNAS middleware via replication.query.
func ListReplications(ctx context.Context, c *Client) ([]ReplicationTask, error) {
	raw, err := c.Call(ctx, "replication.query", []any{})
	if err != nil {
		return nil, fmt.Errorf("replication.query: %w", err)
	}

	var rawList []rawReplication
	if err := json.Unmarshal(raw, &rawList); err != nil {
		return nil, fmt.Errorf("replication.query: decode: %w", err)
	}

	tasks := make([]ReplicationTask, len(rawList))
	for i, r := range rawList {
		targetPool := ""
		if r.TargetDataset != "" {
			parts := strings.Split(r.TargetDataset, "/")
			targetPool = parts[0]
		}

		srcPoolSet := make(map[string]struct{})
		for _, ds := range r.SourceDatasets {
			parts := strings.Split(ds, "/")
			if len(parts) > 0 && parts[0] != "" {
				srcPoolSet[parts[0]] = struct{}{}
			}
		}
		sourcePools := make([]string, 0, len(srcPoolSet))
		for p := range srcPoolSet {
			sourcePools = append(sourcePools, p)
		}
		sort.Strings(sourcePools)

		srcDatasets := r.SourceDatasets
		if srcDatasets == nil {
			srcDatasets = []string{}
		}

		task := ReplicationTask{
			ID:             r.ID,
			Name:           r.Name,
			Direction:      r.Direction,
			Transport:      r.Transport,
			SourceDatasets: srcDatasets,
			TargetDataset:  r.TargetDataset,
			TargetPool:     targetPool,
			SourcePools:    sourcePools,
			State:          r.State.State,
			LastSnapshot:   r.State.LastSnapshot,
			Enabled:        r.Enabled,
		}

		if r.Job != nil {
			task.JobState = r.Job.State
			if r.Job.Progress != nil {
				task.ProgressPercent = r.Job.Progress.Percent
				task.ProgressDescription = r.Job.Progress.Description
			}
			task.TimeStarted = parseTrueNASTime(r.Job.TimeStarted)
			task.TimeFinished = parseTrueNASTime(r.Job.TimeFinished)
			if task.TimeStarted != nil && task.TimeFinished != nil && *task.TimeFinished >= *task.TimeStarted {
				dur := *task.TimeFinished - *task.TimeStarted
				task.DurationSeconds = &dur
			}
		}

		if task.TimeFinished == nil {
			task.TimeFinished = parseTrueNASTime(r.State.Datetime)
		}

		tasks[i] = task
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

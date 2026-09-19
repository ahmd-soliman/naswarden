package truenas

import (
	"encoding/json"
	"testing"
)

func TestDecodeReplication(t *testing.T) {
	fixture := `[
		{
			"id": 24,
			"name": "P1_BKP",
			"direction": "PUSH",
			"transport": "LOCAL",
			"source_datasets": ["P1/main", "P1/user-home"],
			"target_dataset": "S_BKP/P1",
			"enabled": true,
			"state": {
				"state": "FINISHED",
				"last_snapshot": "P1/user-home@auto-2026-09-19_00-00",
				"datetime": {"$date": 1789770304000}
			},
			"job": {
				"id": 13972,
				"state": "SUCCESS",
				"time_started": {"$date": 1789768803000},
				"time_finished": {"$date": 1789770304000},
				"progress": {
					"percent": 100,
					"description": "Sending 7 of 7: P1/user-home@auto-2026-09-19_00-00"
				}
			}
		}
	]`

	var rawList []rawReplication
	if err := json.Unmarshal([]byte(fixture), &rawList); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	if len(rawList) != 1 {
		t.Fatalf("expected 1 task, got %d", len(rawList))
	}

	r := rawList[0]
	if r.Name != "P1_BKP" || r.TargetDataset != "S_BKP/P1" {
		t.Errorf("unexpected fields: %+v", r)
	}
	if r.Job == nil || r.Job.State != "SUCCESS" {
		t.Errorf("expected job state SUCCESS, got %+v", r.Job)
	}
}

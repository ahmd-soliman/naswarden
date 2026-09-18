package truenas

import (
	"context"
	"encoding/json"
	"fmt"
)

// Pool is the subset of pool.query's response naswarden cares about.
// Field names/shape confirmed directly against a live TrueNAS SCALE
// 25.10.7 box via `midclt call pool.query` -- not guessed from docs.
type Pool struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // ONLINE / DEGRADED / FAULTED / ...
	Healthy   bool   `json:"healthy"`
	Warning   bool   `json:"warning"`
	Size      int64  `json:"size"`
	Allocated int64  `json:"allocated"`
	Free      int64  `json:"free"`
	Scan      *Scan  `json:"scan"`
}

type Scan struct {
	Function string `json:"function"` // SCRUB / RESILVER / ...
	State    string `json:"state"`    // FINISHED / IN_PROGRESS / ...
	Errors   int64  `json:"errors"`
}

// ListPools calls pool.query with no filters and returns every pool.
func ListPools(ctx context.Context, c *Client) ([]Pool, error) {
	raw, err := c.Call(ctx, "pool.query", []any{})
	if err != nil {
		return nil, fmt.Errorf("pool.query: %w", err)
	}
	var pools []Pool
	if err := json.Unmarshal(raw, &pools); err != nil {
		return nil, fmt.Errorf("pool.query: decode: %w", err)
	}
	return pools, nil
}

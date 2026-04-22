package game

import (
	"context"
	"fmt"

	"bahago/internal/database/db"
)

// TravelTicks returns the number of ticks required to travel between two world
// coordinates. Uses Chebyshev distance (max of horizontal/vertical delta) with
// a minimum of 12 ticks (3 hours) so nearby kingdoms still require real travel time.
func TravelTicks(x1, y1, x2, y2 int) int {
	dx := x1 - x2
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y2
	if dy < 0 {
		dy = -dy
	}
	d := dx
	if dy > d {
		d = dy
	}
	if d < 12 {
		d = 12
	}
	return d
}

// AdvanceCampaigns decrements all campaigns and transitions those that hit zero:
// en_route → active → returning → deleted
func AdvanceCampaigns(ctx context.Context, q *db.Queries) error {
	atZero, err := q.DecrementAndListCampaignsAtZero(ctx)
	if err != nil {
		return fmt.Errorf("campaigns: decrement: %w", err)
	}

	var toActivate, toReturn, toDelete []int
	for _, m := range atZero {
		switch m.Status {
		case "en_route":
			toActivate = append(toActivate, m.ID)
		case "active":
			toReturn = append(toReturn, m.ID)
		case "returning":
			toDelete = append(toDelete, m.ID)
		}
	}
	if len(toActivate) > 0 {
		if err := q.BulkActivateCampaigns(ctx, toActivate); err != nil {
			return fmt.Errorf("campaigns: bulk activate: %w", err)
		}
	}
	if len(toReturn) > 0 {
		if err := q.BulkReturnCampaigns(ctx, toReturn); err != nil {
			return fmt.Errorf("campaigns: bulk return: %w", err)
		}
	}
	if len(toDelete) > 0 {
		if err := q.BulkDeleteCampaigns(ctx, toDelete); err != nil {
			return fmt.Errorf("campaigns: bulk delete: %w", err)
		}
	}
	return nil
}

package monitor

import (
	"database/sql"
	"testing"

	"github.com/buckelew/go-monitor/internal/database"
)

func TestShouldNotify(t *testing.T) {
	storeSub := database.TaskSubscription{
		Mode: "all",
	}
	newSub := database.TaskSubscription{
		Mode: "new",
	}
	restockSub := database.TaskSubscription{
		Mode:   "restock",
		ItemID: sql.NullInt32{Int32: 42, Valid: true},
	}

	tests := []struct {
		name   string
		sub    database.TaskSubscription
		event  ItemEvent
		expect bool
	}{
		// "all" mode matches everything
		{"all matches new_product", storeSub, ItemEvent{Type: EventNewProduct}, true},
		{"all matches restock", storeSub, ItemEvent{Type: EventRestock}, true},
		{"all matches delisted", storeSub, ItemEvent{Type: EventDelisted}, true},

		// "new" mode matches only new_product
		{"new matches new_product", newSub, ItemEvent{Type: EventNewProduct}, true},
		{"new ignores restock", newSub, ItemEvent{Type: EventRestock}, false},
		{"new ignores delisted", newSub, ItemEvent{Type: EventDelisted}, false},

		// "restock" mode matches restock+delisted when item_id matches
		{"restock matches restock with matching item", restockSub, ItemEvent{Type: EventRestock, ItemID: 42}, true},
		{"restock matches delisted with matching item", restockSub, ItemEvent{Type: EventDelisted, ItemID: 42}, true},
		{"restock ignores new_product", restockSub, ItemEvent{Type: EventNewProduct, ItemID: 42}, false},
		{"restock ignores wrong item_id", restockSub, ItemEvent{Type: EventRestock, ItemID: 99}, false},
		{"restock ignores zero item_id", restockSub, ItemEvent{Type: EventRestock, ItemID: 0}, false},

		// restock with invalid item_id never matches
		{"restock with null item_id never matches", database.TaskSubscription{
			Mode:   "restock",
			ItemID: sql.NullInt32{Valid: false},
		}, ItemEvent{Type: EventRestock, ItemID: 42}, false},

		// unknown mode
		{"unknown mode never matches", database.TaskSubscription{Mode: "bogus"}, ItemEvent{Type: EventNewProduct}, true == false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldNotify(tt.sub, tt.event)
			if got != tt.expect {
				t.Errorf("ShouldNotify() = %v, want %v", got, tt.expect)
			}
		})
	}
}

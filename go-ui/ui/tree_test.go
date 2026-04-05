package ui

import (
	"testing"

	"netFlow_tool-ui/types"
)

func TestBuildProcessTreeRowsNestsChildrenUnderParent(t *testing.T) {
	parentPID := uint32(10)
	rows := buildProcessTreeRows([]types.ProcessFlow{
		{PID: 10, Name: "parent", DownloadSpeed: 100},
		{PID: 11, ParentPID: &parentPID, Name: "child", DownloadSpeed: 50},
	}, "", "download", false, map[uint32]bool{10: true})

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if !rows[0].HasChildren || !rows[0].Selectable || !rows[0].Expanded {
		t.Fatalf("expected parent row to be expandable: %+v", rows[0])
	}
	if rows[0].AggregateDownloadSpeed != 150 {
		t.Fatalf("expected parent aggregate download speed 150, got %f", rows[0].AggregateDownloadSpeed)
	}
	if rows[1].Depth != 1 || rows[1].Flow.PID != 11 {
		t.Fatalf("expected child row nested under parent: %+v", rows[1])
	}
}

func TestPaginateRowsClampsPageIndex(t *testing.T) {
	rows := []int{1, 2, 3, 4, 5}
	page, pageIndex, totalPages := paginateRows(rows, 9, 2)

	if totalPages != 3 {
		t.Fatalf("expected 3 total pages, got %d", totalPages)
	}
	if pageIndex != 2 {
		t.Fatalf("expected clamped page index 2, got %d", pageIndex)
	}
	if len(page) != 1 || page[0] != 5 {
		t.Fatalf("unexpected paged rows: %+v", page)
	}
}

func TestSortProcessFlowsAscendingByName(t *testing.T) {
	rows := buildProcessTreeRows([]types.ProcessFlow{
		{PID: 2, Name: "bravo"},
		{PID: 1, Name: "alpha"},
	}, "", "name", true, map[uint32]bool{})

	if len(rows) != 2 || rows[0].Flow.Name != "alpha" || rows[1].Flow.Name != "bravo" {
		t.Fatalf("expected ascending name sort, got %+v", rows)
	}
}

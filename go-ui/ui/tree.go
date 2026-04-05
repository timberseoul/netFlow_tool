package ui

import (
	"cmp"
	"sort"

	"netFlow_tool-ui/types"
)

type processTreeRow struct {
	Flow                   types.ProcessFlow
	Depth                  int
	HasChildren            bool
	Expanded               bool
	Selectable             bool
	AggregateUploadSpeed   float64
	AggregateDownloadSpeed float64
	AggregateTotalUpload   uint64
	AggregateTotalDownload uint64
}

type processTreeNode struct {
	flow     types.ProcessFlow
	children []*processTreeNode
}

func sortProcessFlows(flows []types.ProcessFlow, sortBy string, sortAsc bool) []types.ProcessFlow {
	sorted := make([]types.ProcessFlow, len(flows))
	copy(sorted, flows)

	sort.Slice(sorted, func(i, j int) bool {
		var comparison int
		switch sortBy {
		case "download":
			comparison = cmp.Compare(sorted[i].DownloadSpeed, sorted[j].DownloadSpeed)
		case "upload":
			comparison = cmp.Compare(sorted[i].UploadSpeed, sorted[j].UploadSpeed)
		case "name":
			comparison = cmp.Compare(sorted[i].Name, sorted[j].Name)
		case "pid":
			comparison = cmp.Compare(sorted[i].PID, sorted[j].PID)
		default:
			comparison = cmp.Compare(sorted[i].DownloadSpeed, sorted[j].DownloadSpeed)
		}
		if comparison == 0 {
			return sorted[i].PID < sorted[j].PID
		}
		if sortAsc {
			return comparison < 0
		}
		return comparison > 0
	})

	return sorted
}

func buildProcessTreeRows(
	flows []types.ProcessFlow,
	filterCategory string,
	sortBy string,
	sortAsc bool,
	expanded map[uint32]bool,
) []processTreeRow {
	filtered := make([]types.ProcessFlow, 0, len(flows))
	for _, flow := range flows {
		if filterCategory != "" && flow.Category != filterCategory {
			continue
		}
		filtered = append(filtered, flow)
	}

	sorted := sortProcessFlows(filtered, sortBy, sortAsc)
	nodes := make(map[uint32]*processTreeNode, len(sorted))
	order := make([]*processTreeNode, 0, len(sorted))
	for _, flow := range sorted {
		node := &processTreeNode{flow: flow}
		nodes[flow.PID] = node
		order = append(order, node)
	}

	roots := make([]*processTreeNode, 0, len(order))
	for _, node := range order {
		if node.flow.ParentPID != nil {
			if parent, ok := nodes[*node.flow.ParentPID]; ok && parent.flow.PID != node.flow.PID {
				parent.children = append(parent.children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	sortTreeNodes(roots, sortBy, sortAsc)

	rows := make([]processTreeRow, 0, len(sorted))
	for _, root := range roots {
		appendProcessTreeRows(&rows, root, 0, expanded)
	}

	return rows
}

func sortTreeNodes(nodes []*processTreeNode, sortBy string, sortAsc bool) {
	sort.SliceStable(nodes, func(i, j int) bool {
		leftUploadSpeed, leftDownloadSpeed, leftTotalUpload, leftTotalDownload := aggregateProcessNode(nodes[i])
		rightUploadSpeed, rightDownloadSpeed, rightTotalUpload, rightTotalDownload := aggregateProcessNode(nodes[j])

		var comparison int
		switch sortBy {
		case "download":
			comparison = cmp.Compare(leftDownloadSpeed, rightDownloadSpeed)
		case "upload":
			comparison = cmp.Compare(leftUploadSpeed, rightUploadSpeed)
		case "name":
			comparison = cmp.Compare(nodes[i].flow.Name, nodes[j].flow.Name)
		case "pid":
			comparison = cmp.Compare(nodes[i].flow.PID, nodes[j].flow.PID)
		case "total_upload":
			comparison = cmp.Compare(leftTotalUpload, rightTotalUpload)
		case "total_download":
			comparison = cmp.Compare(leftTotalDownload, rightTotalDownload)
		default:
			comparison = cmp.Compare(leftDownloadSpeed, rightDownloadSpeed)
		}

		if comparison == 0 {
			comparison = cmp.Compare(nodes[i].flow.PID, nodes[j].flow.PID)
		}
		if sortAsc {
			return comparison < 0
		}
		return comparison > 0
	})

	for _, node := range nodes {
		if len(node.children) > 0 {
			sortTreeNodes(node.children, sortBy, sortAsc)
		}
	}
}

func appendProcessTreeRows(rows *[]processTreeRow, node *processTreeNode, depth int, expanded map[uint32]bool) {
	uploadSpeed, downloadSpeed, totalUpload, totalDownload := aggregateProcessNode(node)
	hasChildren := len(node.children) > 0
	isExpanded := hasChildren && expanded[node.flow.PID]
	*rows = append(*rows, processTreeRow{
		Flow:                   node.flow,
		Depth:                  depth,
		HasChildren:            hasChildren,
		Expanded:               isExpanded,
		Selectable:             hasChildren,
		AggregateUploadSpeed:   uploadSpeed,
		AggregateDownloadSpeed: downloadSpeed,
		AggregateTotalUpload:   totalUpload,
		AggregateTotalDownload: totalDownload,
	})

	if !isExpanded {
		return
	}

	for _, child := range node.children {
		appendProcessTreeRows(rows, child, depth+1, expanded)
	}
}

func aggregateProcessNode(node *processTreeNode) (float64, float64, uint64, uint64) {
	uploadSpeed := node.flow.UploadSpeed
	downloadSpeed := node.flow.DownloadSpeed
	totalUpload := node.flow.TotalUpload
	totalDownload := node.flow.TotalDownload

	for _, child := range node.children {
		childUploadSpeed, childDownloadSpeed, childTotalUpload, childTotalDownload := aggregateProcessNode(child)
		uploadSpeed += childUploadSpeed
		downloadSpeed += childDownloadSpeed
		totalUpload += childTotalUpload
		totalDownload += childTotalDownload
	}

	return uploadSpeed, downloadSpeed, totalUpload, totalDownload
}

func paginateRows[T any](rows []T, pageIndex, pageSize int) ([]T, int, int) {
	if pageSize <= 0 {
		pageSize = 1
	}
	totalPages := (len(rows) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if pageIndex < 0 {
		pageIndex = 0
	}
	if pageIndex >= totalPages {
		pageIndex = totalPages - 1
	}

	start := pageIndex * pageSize
	if start > len(rows) {
		start = len(rows)
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}

	return rows[start:end], pageIndex, totalPages
}

func clampPageIndex(pageIndex, totalItems, pageSize int) int {
	_, normalized, _ := paginateRows(make([]struct{}, totalItems), pageIndex, pageSize)
	return normalized
}

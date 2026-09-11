package domain

import (
	"sort"
)

type PVEVMCopies struct {
	VMID   int64
	VMName string
	PBS    int
	Vzdump int
	Total  int
}

// Count only retained backup entries in the latest PVE storage inventory.
func CountPVEVMCopies(storages []PVEStorage, contents map[int64][]PVEStorageContent) []PVEVMCopies {
	byVM := make(map[int64]*PVEVMCopies)
	for _, storage := range storages {
		seen := make(map[string]bool)
		for _, item := range contents[storage.ID] {
			if item.Content != "backup" || item.VMID <= 0 {
				continue
			}
			if item.VolID != "" {
				if seen[item.VolID] {
					continue
				}
				seen[item.VolID] = true
			}
			row := byVM[item.VMID]
			if row == nil {
				row = &PVEVMCopies{VMID: item.VMID}
				byVM[item.VMID] = row
			}
			if storage.Type == "pbs" {
				row.PBS++
			} else {
				row.Vzdump++
			}
			row.Total++
		}
	}
	rows := make([]PVEVMCopies, 0, len(byVM))
	for _, row := range byVM {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].VMID < rows[j].VMID })
	return rows
}

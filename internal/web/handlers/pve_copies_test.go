package webhandlers

import (
	"fmt"
	"testing"

	"probakgo/internal/domain"
)

func TestCountPVEVMCopies(t *testing.T) {
	storages := []domain.PVEStorage{{ID: 1, Type: "pbs"}, {ID: 2, Type: "dir"}, {ID: 3, Type: "nfs"}}
	contents := map[int64][]domain.PVEStorageContent{}
	for i := 0; i < 20; i++ {
		contents[1] = append(contents[1], domain.PVEStorageContent{VMID: 100, Content: "backup", VolID: fmt.Sprintf("pbs:backup/vm/100/%d", i)})
	}
	for i := 0; i < 3; i++ {
		contents[2] = append(contents[2], domain.PVEStorageContent{VMID: 100, Content: "backup", VolID: fmt.Sprintf("local:backup/%d", i)})
	}
	contents[1] = append(contents[1], contents[1][0]) // Duplicate inventory entry.
	contents[2] = append(contents[2], domain.PVEStorageContent{VMID: 100, Content: "images"}, domain.PVEStorageContent{Content: "backup"})
	contents[3] = []domain.PVEStorageContent{{VMID: 101, Content: "backup", VolID: "nfs:backup/101"}}
	rows := countPVEVMCopies(storages, contents)
	if len(rows) != 2 || rows[0] != (pveVMCopies{VMID: 100, PBS: 20, Vzdump: 3, Total: 23}) || rows[1] != (pveVMCopies{VMID: 101, Vzdump: 1, Total: 1}) {
		t.Fatalf("unexpected copy counts: %+v", rows)
	}
	if rows := countPVEVMCopies(nil, nil); len(rows) != 0 {
		t.Fatalf("empty inventory: %+v", rows)
	}
}

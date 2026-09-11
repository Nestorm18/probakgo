package webhandlers

import "probakgo/internal/domain"

type pveVMCopies = domain.PVEVMCopies

func countPVEVMCopies(storages []domain.PVEStorage, contents map[int64][]domain.PVEStorageContent) []pveVMCopies {
	return domain.CountPVEVMCopies(storages, contents)
}

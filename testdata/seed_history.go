//go:build ignore

// seed_history inserts 6 days of historical reports into the SQLite DB
// so the PVE/PBS report history pages and charts have realistic data.
//
// Run from the project root (after seed.sh has created the servers):
//
//	go run testdata/seed_history.go
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := resolveDBPath()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("db not reachable (%s): %v", dbPath, err)
	}

	var pveID, pbsID int64
	if err := db.QueryRow("SELECT id FROM pve_servers WHERE name='soporte1' AND is_deleted=0").Scan(&pveID); err != nil {
		log.Fatal("soporte1 not found - run seed.sh first")
	}
	if err := db.QueryRow("SELECT id FROM pbs_servers WHERE name='pbs-test' AND is_deleted=0").Scan(&pbsID); err != nil {
		log.Fatal("pbs-test not found - run seed.sh first")
	}

	fmt.Printf("DB     : %s\n", dbPath)
	fmt.Printf("PVE ID : %d (soporte1)\n", pveID)
	fmt.Printf("PBS ID : %d (pbs-test)\n", pbsID)
	fmt.Println()

	type day struct {
		offset   int
		status   string
		duration int64
		pbsUsed  int64
	}

	// Today's report is already created by seed.sh.
	// Insert days -6 to -1, varying statuses and durations.
	days := []day{
		{6, "OK", 1180, 2650000000000},
		{5, "OK", 1220, 2680000000000},
		{4, "warning", 1567, 2710000000000},
		{3, "OK", 1190, 2730000000000},
		{2, "error", 0, 2750000000000},
		{1, "OK", 1245, 2760000000000},
	}

	for _, d := range days {
		ts := time.Now().AddDate(0, 0, -d.offset).Truncate(24*time.Hour).Add(8 * time.Hour)

		if err := insertPVEDay(db, pveID, ts, d.status, d.duration); err != nil {
			log.Fatalf("PVE day -%d: %v", d.offset, err)
		}
		if err := insertPBSDay(db, pbsID, ts, d.pbsUsed); err != nil {
			log.Fatalf("PBS day -%d: %v", d.offset, err)
		}

		fmt.Printf("día -%d  %s  PVE: %-7s %4ds  PBS: %.2f TB\n",
			d.offset, ts.Format("2006-01-02"),
			d.status, d.duration, float64(d.pbsUsed)/1e12)
	}

	fmt.Println()
	fmt.Println("Listo. Abre el historial de reportes en el dashboard.")
}

func insertPVEDay(db *sql.DB, serverID int64, ts time.Time, status string, duration int64) error {
	starttime := ts.Unix() - duration
	endtime := ts.Unix()

	res, err := db.Exec(
		`INSERT INTO pve_reports (server_id, reported_at, backup_status, backup_starttime, backup_endtime, backup_duration)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		serverID, ts.Format("2006-01-02 15:04:05"), status, starttime, endtime, duration,
	)
	if err != nil {
		return err
	}
	reportID, _ := res.LastInsertId()

	// NAS (backup)
	nasID, err := insertStorage(db, reportID,
		"NAS", "/mnt/pve/NAS", "backup", "nfs", "available", true, "192.168.1.100",
		"abc123def456", `{"keep-daily":7,"keep-weekly":4,"keep-monthly":3}`)
	if err != nil {
		return err
	}
	if err := insertStorageInfo(db, nasID, 10737418240000, 4294967296000, 6442450944000, 40.0); err != nil {
		return err
	}
	for _, c := range nasContent(ts) {
		if err := insertContent(db, nasID, c); err != nil {
			return err
		}
	}

	// NAS-GRANDE (backup + iso)
	ngID, err := insertStorage(db, reportID,
		"NAS-GRANDE", "/mnt/pve/NAS-GRANDE", "backup,iso", "nfs", "available", true, "192.168.1.101",
		"fedcba987654", `{"keep-daily":14}`)
	if err != nil {
		return err
	}
	if err := insertStorageInfo(db, ngID, 32212254720000, 9663676416000, 22548578304000, 30.0); err != nil {
		return err
	}
	for _, c := range nasGrandeContent(ts) {
		if err := insertContent(db, ngID, c); err != nil {
			return err
		}
	}

	// local-lvm (no backups)
	lvmID, err := insertStorage(db, reportID,
		"local-lvm", "", "rootdir,images", "lvmthin", "available", false, "",
		"111aaa222bbb", `{}`)
	if err != nil {
		return err
	}
	return insertStorageInfo(db, lvmID, 107374182400, 53687091200, 53687091200, 50.0)
}

func insertStorage(db *sql.DB, reportID int64, name, path, content, typ, status string, shared bool, server, digest, pruneJSON string) (int64, error) {
	sharedInt := 0
	if shared {
		sharedInt = 1
	}
	res, err := db.Exec(
		`INSERT INTO pve_storages (report_id, storage, path, content, type, status, shared, server, digest, prune_backups)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reportID, name, path, content, typ, status, sharedInt, server, digest, pruneJSON,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertStorageInfo(db *sql.DB, storageID, total, used, avail int64, usedPct float64) error {
	_, err := db.Exec(
		`INSERT INTO pve_storage_info (storage_id, total, used, avail, used_percent, active, enabled, lvl)
		 VALUES (?, ?, ?, ?, ?, 1, 1, 0)`,
		storageID, total, used, avail, usedPct,
	)
	return err
}

type contentRow struct {
	vmid    int
	format  string
	size    int64
	subtype string
	volid   string
	notes   string
}

func nasContent(ts time.Time) []contentRow {
	d := ts.Format("2006_01_02")
	return []contentRow{
		{100, "vma.zst", 5368709120, "qemu", fmt.Sprintf("NAS:backup/vzdump-qemu-100-%s-08_00_00.vma.zst", d), "servidor-web"},
		{101, "vma.zst", 3221225472, "qemu", fmt.Sprintf("NAS:backup/vzdump-qemu-101-%s-08_21_00.vma.zst", d), "base-de-datos"},
		{102, "vma.zst", 8589934592, "qemu", fmt.Sprintf("NAS:backup/vzdump-qemu-102-%s-08_45_00.vma.zst", d), "servidor-ldap"},
		{200, "tar.zst", 1073741824, "lxc", fmt.Sprintf("NAS:backup/vzdump-lxc-200-%s-09_00_00.vma.zst", d), "proxy-nginx"},
	}
}

func nasGrandeContent(ts time.Time) []contentRow {
	d := ts.Format("2006_01_02")
	return []contentRow{
		{103, "vma.zst", 12884901888, "qemu", fmt.Sprintf("NAS-GRANDE:backup/vzdump-qemu-103-%s-09_15_00.vma.zst", d), "servidor-erp"},
	}
}

func insertContent(db *sql.DB, storageID int64, c contentRow) error {
	_, err := db.Exec(
		`INSERT INTO pve_storage_content (storage_id, vmid, format, size, content, volid, ctime, subtype, notes)
		 VALUES (?, ?, ?, ?, 'backup', ?, ?, ?, ?)`,
		storageID, c.vmid, c.format, c.size, c.volid, time.Now().Unix(), c.subtype, c.notes,
	)
	return err
}

func insertPBSDay(db *sql.DB, serverID int64, ts time.Time, used int64) error {
	const total = int64(2948636082176)
	avail := total - used

	res, err := db.Exec(
		`INSERT INTO pbs_reports (server_id, reported_at) VALUES (?, ?)`,
		serverID, ts.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return err
	}
	reportID, _ := res.LastInsertId()

	res, err = db.Exec(
		`INSERT INTO pbs_stores (report_id, store, total, used, avail, estimated_full_date, mount_status, history_start, history_delta)
		 VALUES (?, 'synology', ?, ?, ?, 1800000000, 'nonremovable', 1745500000, 1800)`,
		reportID, total, used, avail,
	)
	if err != nil {
		return err
	}
	storeID, _ := res.LastInsertId()

	usedPct := float64(used) / float64(total)
	for i := 0; i < 10; i++ {
		v := usedPct - float64(9-i)*0.01
		if _, err := db.Exec(`INSERT INTO pbs_store_history (store_id, position, value) VALUES (?, ?, ?)`, storeID, i, v); err != nil {
			return err
		}
	}

	_, err = db.Exec(
		`INSERT INTO pbs_gc_status (store_id, disk_bytes, disk_chunks, index_data_bytes, index_file_count,
		 pending_bytes, pending_chunks, removed_bad, removed_bytes, removed_chunks, still_bad, upid)
		 VALUES (?, 1917425818843, 967294, 95412725514240, 717, 10715476825, 5963, 0, 7624202312, 3790, 0,
		 'UPID:pbs:000002DE:00001122:000006A2:692518E0:garbage_collection:synology:root@pam:')`,
		storeID,
	)
	return err
}

func resolveDBPath() string {
	dbPath := "probakgo_data.db"

	f, err := os.Open(".env")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "DATABASE_PATH=") {
				val := strings.TrimPrefix(line, "DATABASE_PATH=")
				val = strings.Trim(val, `"'`)
				val = strings.TrimPrefix(val, "./")
				dbPath = val
				break
			}
		}
	}

	return dbPath
}

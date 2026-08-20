package webhandlers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/xlsx"
)

const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type pveWorkbookServer struct {
	Server  domain.PVEServer
	Report  domain.PVEReport
	Tasks   []domain.PVEBackupTask
	Current bool
}

type pbsWorkbookServer struct {
	Server  domain.PBSServer
	Report  domain.PBSReport
	Stores  []domain.PBSStore
	Tasks   []domain.PBSTask
	Current bool
}

type windowsWorkbookServer struct {
	Server    domain.WindowsServer
	Report    domain.WindowsReport
	Heartbeat domain.ServerHeartbeat
	Disks     []domain.WindowsDisk
	Current   bool
}

func (h *WebH) PVEServersXLSX(w http.ResponseWriter, r *http.Request) {
	workbook, err := h.buildPVEWorkbook(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveXLSX(w, "informe_pve_"+time.Now().Format("20060102")+".xlsx", workbook)
}

func (h *WebH) PBSServersXLSX(w http.ResponseWriter, r *http.Request) {
	workbook, err := h.buildPBSWorkbook(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveXLSX(w, "informe_pbs_"+time.Now().Format("20060102")+".xlsx", workbook)
}

func (h *WebH) WindowsServersXLSX(w http.ResponseWriter, r *http.Request) {
	workbook, err := h.buildWindowsWorkbook(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveXLSX(w, "informe_windows_"+time.Now().Format("20060102")+".xlsx", workbook)
}

func (h *WebH) buildPVEWorkbook(r *http.Request) (xlsx.Workbook, error) {
	ctx := r.Context()
	servers, err := h.store.ListPVEServers(ctx)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	reports, err := h.store.GetLatestPVEReports(ctx)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	generatedAt := reportNow(h)
	reportIDs := make([]int64, 0, len(reports))
	for _, report := range reports {
		reportIDs = append(reportIDs, report.ID)
	}
	tasksByReport, err := h.store.GetPVEBackupTasksForReports(ctx, reportIDs)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	included := make([]pveWorkbookServer, 0, len(servers))
	for _, server := range servers {
		report := reports[server.ID]
		if report == nil {
			continue
		}
		included = append(included, pveWorkbookServer{Server: server, Report: *report, Tasks: latestPVETasks(tasksByReport[report.ID]), Current: reportIsFromToday(report.ReportedAt, generatedAt)})
	}
	return pveWorkbook(included, generatedAt), nil
}

func (h *WebH) buildPBSWorkbook(r *http.Request) (xlsx.Workbook, error) {
	ctx := r.Context()
	servers, err := h.store.ListPBSServers(ctx)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	reports, err := h.store.GetLatestPBSReports(ctx)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	generatedAt := reportNow(h)
	reportIDs := make([]int64, 0, len(reports))
	for _, report := range reports {
		reportIDs = append(reportIDs, report.ID)
	}
	storesByReport, err := h.store.GetPBSStoresForReports(ctx, reportIDs)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	tasksByReport, err := h.store.GetPBSTasksForReports(ctx, reportIDs)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	included := make([]pbsWorkbookServer, 0, len(servers))
	for _, server := range servers {
		report := reports[server.ID]
		if report == nil {
			continue
		}
		included = append(included, pbsWorkbookServer{
			Server: server, Report: *report, Stores: storesByReport[report.ID], Tasks: tasksByReport[report.ID], Current: reportIsFromToday(report.ReportedAt, generatedAt),
		})
	}
	return pbsWorkbook(included, generatedAt), nil
}

func (h *WebH) buildWindowsWorkbook(r *http.Request) (xlsx.Workbook, error) {
	ctx := r.Context()
	servers, err := h.store.ListWindowsServers(ctx)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	reports, err := h.store.GetLatestWindowsReports(ctx)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	heartbeats, _ := h.store.ListServerHeartbeatsByType(ctx, "windows")
	generatedAt := reportNow(h)
	reportIDs := make([]int64, 0, len(reports))
	for _, report := range reports {
		reportIDs = append(reportIDs, report.ID)
	}
	disksByReport, err := h.store.GetWindowsDisksForReports(ctx, reportIDs)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	included := make([]windowsWorkbookServer, 0, len(servers))
	for _, server := range servers {
		report := reports[server.ID]
		heartbeat := heartbeats[server.ID]
		if report == nil {
			continue
		}
		included = append(included, windowsWorkbookServer{
			Server: server, Report: *report, Heartbeat: heartbeat, Disks: disksByReport[report.ID], Current: reportIsFromToday(report.ReportedAt, generatedAt),
		})
	}
	return windowsWorkbook(included, generatedAt), nil
}

func pveWorkbook(servers []pveWorkbookServer, generatedAt time.Time) xlsx.Workbook {
	detailRows := [][]xlsx.Cell{
		titleRow("Última copia por VM · PVE", 11),
		generatedRow(generatedAt),
		{},
		headerRow("Servidor", "Hostname", "IP", "VMID", "VM", "Estado", "Fecha", "Vigencia", "Duración", "Tamaño", "Ruta"),
	}
	serverRows := make([][]xlsx.Cell, 0, len(servers))
	totalTasks, totalSize, totalDuration := 0, int64(0), int64(0)
	for _, server := range servers {
		okCount, warningCount, errorCount := 0, 0, 0
		serverSize, serverDuration := int64(0), int64(0)
		for _, task := range server.Tasks {
			style := backupStatusStyle(task.Status)
			detailRows = append(detailRows, []xlsx.Cell{
				xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.Integer(task.VMID),
				xlsx.Text(task.VMName), xlsx.StyledText(task.Status, style), xlsx.DateTime(time.Unix(task.StartTime, 0)),
				reportFreshnessCell(server.Current), xlsx.Duration(task.Duration), xlsx.Bytes(task.Size), xlsx.Text(task.Filename),
			})
			totalTasks++
			totalSize += task.Size
			totalDuration += task.Duration
			serverSize += task.Size
			serverDuration += task.Duration
			switch style {
			case xlsx.StyleOK:
				okCount++
			case xlsx.StyleWarning:
				warningCount++
			default:
				errorCount++
			}
		}
		serverRows = append(serverRows, []xlsx.Cell{
			xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.DateTime(server.Report.ReportedAt),
			reportFreshnessCell(server.Current), xlsx.Integer(int64(len(server.Tasks))), xlsx.Integer(int64(okCount)), xlsx.Integer(int64(warningCount)), xlsx.Integer(int64(errorCount)),
			xlsx.Bytes(serverSize), xlsx.Duration(serverDuration),
		})
	}
	summaryRows := summaryStart("Informe de copias PVE", generatedAt,
		[]summaryMetric{{"PVE con datos", float64(len(servers)), xlsx.StyleInteger}, {"VM con copia", float64(totalTasks), xlsx.StyleInteger}, {"Tamaño total", float64(totalSize), xlsx.StyleBytes}, {"Duración total", float64(totalDuration) / 86400, xlsx.StyleDuration}}, 11)
	summaryRows = append(summaryRows, sectionRow("Resumen por servidor", 11), headerRow("Servidor", "Hostname", "IP", "Último reporte", "Vigencia", "VM", "OK", "Avisos", "Errores", "Tamaño", "Duración"))
	summaryRows = append(summaryRows, serverRows...)
	return xlsx.Workbook{Sheets: []xlsx.Sheet{
		{Name: "Resumen", Rows: summaryRows, ColumnWidths: []float64{24, 22, 16, 20, 18, 11, 11, 11, 11, 16, 15}, FreezeRows: 7, AutoFilter: tableFilter(7, len(summaryRows), 11), Merges: []string{"A1:K1", "A6:K6"}},
		{Name: "Últimas copias", Rows: detailRows, ColumnWidths: []float64{24, 22, 16, 11, 24, 18, 20, 18, 15, 16, 58}, FreezeRows: 4, AutoFilter: tableFilter(4, len(detailRows), 11), Merges: []string{"A1:K1"}},
	}}
}

func pbsWorkbook(servers []pbsWorkbookServer, generatedAt time.Time) xlsx.Workbook {
	storeRows := [][]xlsx.Cell{titleRow("Datastores · PBS", 12), generatedRow(generatedAt), {}, headerRow("Servidor", "Hostname", "IP", "Último reporte", "Vigencia", "Datastore", "Montaje", "Total", "Usado", "Libre", "Uso", "Llenado estimado")}
	taskRows := [][]xlsx.Cell{titleRow("Últimas tareas de sincronización y GC · PBS", 15), generatedRow(generatedAt), {}, headerRow("Servidor", "Hostname", "IP", "Último reporte", "Vigencia", "Tipo", "Job", "Remoto", "Datastore remoto", "Datastore local", "Estado", "Inicio", "Fin", "Duración", "UPID")}
	summaryServerRows := make([][]xlsx.Cell, 0, len(servers))
	totalStores, totalTasks, totalCapacity, totalUsed := 0, 0, int64(0), int64(0)
	for _, server := range servers {
		serverCapacity, serverUsed := int64(0), int64(0)
		for _, store := range server.Stores {
			usage := float64(0)
			if store.Total > 0 {
				usage = float64(store.Used) / float64(store.Total)
			}
			storeRows = append(storeRows, []xlsx.Cell{
				xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.DateTime(server.Report.ReportedAt),
				reportFreshnessCell(server.Current), xlsx.Text(store.Store), xlsx.Text(store.MountStatus), xlsx.Bytes(store.Total), xlsx.Bytes(store.Used), xlsx.Bytes(store.Avail), xlsx.Percent(usage), xlsx.DateTime(unixTime(store.EstimatedFullDate)),
			})
			totalStores++
			totalCapacity += store.Total
			totalUsed += store.Used
			serverCapacity += store.Total
			serverUsed += store.Used
		}
		for _, task := range server.Tasks {
			statusStyle := xlsx.StyleOK
			if domain.PBSTaskFailed(task) {
				statusStyle = xlsx.StyleError
			}
			taskRows = append(taskRows, []xlsx.Cell{
				xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.DateTime(server.Report.ReportedAt), reportFreshnessCell(server.Current), xlsx.Text(pbsTaskTypeLabel(task.TaskType)),
				xlsx.Text(task.JobID), xlsx.Text(task.Remote), xlsx.Text(task.RemoteStore), xlsx.Text(task.Store), xlsx.StyledText(task.Status, statusStyle),
				xlsx.DateTime(unixTime(task.StartTime)), xlsx.DateTime(unixTime(task.EndTime)), xlsx.Duration(maxInt64(0, task.EndTime-task.StartTime)), xlsx.Text(task.UPID),
			})
			totalTasks++
		}
		usage := float64(0)
		if serverCapacity > 0 {
			usage = float64(serverUsed) / float64(serverCapacity)
		}
		summaryServerRows = append(summaryServerRows, []xlsx.Cell{
			xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.DateTime(server.Report.ReportedAt),
			reportFreshnessCell(server.Current), xlsx.Integer(int64(len(server.Stores))), xlsx.Integer(int64(len(server.Tasks))), xlsx.Bytes(serverCapacity), xlsx.Bytes(serverUsed), xlsx.Percent(usage),
		})
	}
	usage := float64(0)
	if totalCapacity > 0 {
		usage = float64(totalUsed) / float64(totalCapacity)
	}
	summaryRows := summaryStart("Informe de almacenamiento PBS", generatedAt,
		[]summaryMetric{{"PBS con datos", float64(len(servers)), xlsx.StyleInteger}, {"Datastores", float64(totalStores), xlsx.StyleInteger}, {"Tareas", float64(totalTasks), xlsx.StyleInteger}, {"Uso total", usage, xlsx.StylePercent}}, 10)
	summaryRows = append(summaryRows, sectionRow("Resumen por servidor", 10), headerRow("Servidor", "Hostname", "IP", "Último reporte", "Vigencia", "Datastores", "Tareas", "Capacidad", "Usado", "Uso"))
	summaryRows = append(summaryRows, summaryServerRows...)
	return xlsx.Workbook{Sheets: []xlsx.Sheet{
		{Name: "Resumen", Rows: summaryRows, ColumnWidths: []float64{24, 22, 16, 20, 18, 13, 11, 16, 16, 12}, FreezeRows: 7, AutoFilter: tableFilter(7, len(summaryRows), 10), Merges: []string{"A1:J1", "A6:J6"}},
		{Name: "Datastores", Rows: storeRows, ColumnWidths: []float64{24, 22, 16, 20, 18, 24, 16, 16, 16, 16, 12, 20}, FreezeRows: 4, AutoFilter: tableFilter(4, len(storeRows), 12), Merges: []string{"A1:L1"}},
		{Name: "Tareas", Rows: taskRows, ColumnWidths: []float64{24, 22, 16, 20, 18, 18, 22, 22, 22, 22, 20, 20, 20, 15, 58}, FreezeRows: 4, AutoFilter: tableFilter(4, len(taskRows), 15), Merges: []string{"A1:O1"}},
	}}
}

func windowsWorkbook(servers []windowsWorkbookServer, generatedAt time.Time) xlsx.Workbook {
	diskRows := [][]xlsx.Cell{titleRow("Discos · Windows", 14), generatedRow(generatedAt), {}, headerRow("Servidor", "Hostname", "IP", "Último reporte", "Vigencia", "Último heartbeat", "Disco", "Etiqueta", "Sistema", "Tipo", "Estado", "Total", "Usado", "Libre")}
	summaryServerRows := make([][]xlsx.Cell, 0, len(servers))
	totalDisks, totalCapacity, totalUsed := 0, int64(0), int64(0)
	for _, server := range servers {
		serverCapacity, serverUsed := int64(0), int64(0)
		for _, disk := range server.Disks {
			statusStyle := diskHealthStyle(disk.Health)
			diskRows = append(diskRows, []xlsx.Cell{
				xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.DateTime(server.Report.ReportedAt), reportFreshnessCell(server.Current), xlsx.DateTime(server.Heartbeat.LastSeenAt),
				xlsx.Text(disk.Name), xlsx.Text(disk.Label), xlsx.Text(disk.FileSystem), xlsx.Text(disk.DriveType), xlsx.StyledText(disk.Health, statusStyle),
				xlsx.Bytes(disk.Total), xlsx.Bytes(disk.Used), xlsx.Bytes(disk.Free),
			})
			totalDisks++
			totalCapacity += disk.Total
			totalUsed += disk.Used
			serverCapacity += disk.Total
			serverUsed += disk.Used
		}
		usage := float64(0)
		if serverCapacity > 0 {
			usage = float64(serverUsed) / float64(serverCapacity)
		}
		summaryServerRows = append(summaryServerRows, []xlsx.Cell{
			xlsx.Text(server.Server.DisplayName), xlsx.Text(server.Server.Name), xlsx.Text(server.Server.IP), xlsx.DateTime(server.Report.ReportedAt), reportFreshnessCell(server.Current),
			xlsx.DateTime(server.Heartbeat.LastSeenAt), xlsx.Integer(int64(len(server.Disks))), xlsx.Bytes(serverCapacity), xlsx.Bytes(serverUsed), xlsx.Percent(usage),
		})
	}
	usage := float64(0)
	if totalCapacity > 0 {
		usage = float64(totalUsed) / float64(totalCapacity)
	}
	summaryRows := summaryStart("Informe de discos Windows", generatedAt,
		[]summaryMetric{{"Windows con datos", float64(len(servers)), xlsx.StyleInteger}, {"Discos", float64(totalDisks), xlsx.StyleInteger}, {"Capacidad total", float64(totalCapacity), xlsx.StyleBytes}, {"Uso total", usage, xlsx.StylePercent}}, 10)
	summaryRows = append(summaryRows, sectionRow("Resumen por servidor", 10), headerRow("Servidor", "Hostname", "IP", "Último reporte", "Vigencia", "Heartbeat", "Discos", "Capacidad", "Usado", "Uso"))
	summaryRows = append(summaryRows, summaryServerRows...)
	return xlsx.Workbook{Sheets: []xlsx.Sheet{
		{Name: "Resumen", Rows: summaryRows, ColumnWidths: []float64{24, 22, 16, 20, 18, 20, 11, 16, 16, 12}, FreezeRows: 7, AutoFilter: tableFilter(7, len(summaryRows), 10), Merges: []string{"A1:J1", "A6:J6"}},
		{Name: "Discos", Rows: diskRows, ColumnWidths: []float64{24, 22, 16, 20, 18, 20, 14, 20, 14, 15, 18, 16, 16, 16}, FreezeRows: 4, AutoFilter: tableFilter(4, len(diskRows), 14), Merges: []string{"A1:N1"}},
	}}
}

type summaryMetric struct {
	Label string
	Value float64
	Style int
}

func summaryStart(title string, generatedAt time.Time, metrics []summaryMetric, columns int) [][]xlsx.Cell {
	rows := [][]xlsx.Cell{titleRow(title, columns), generatedRow(generatedAt), {}}
	metricRow := make([]xlsx.Cell, 0, len(metrics)*2)
	for _, metric := range metrics {
		metricRow = append(metricRow, xlsx.StyledText(metric.Label, xlsx.StyleSummaryLabel), xlsx.Number(metric.Value, metric.Style))
	}
	rows = append(rows, metricRow, []xlsx.Cell{})
	return rows
}

func titleRow(title string, columns int) []xlsx.Cell {
	row := make([]xlsx.Cell, columns)
	row[0] = xlsx.StyledText(title, xlsx.StyleTitle)
	return row
}

func generatedRow(generatedAt time.Time) []xlsx.Cell {
	return []xlsx.Cell{xlsx.Text("Generado"), xlsx.DateTime(generatedAt)}
}

func sectionRow(title string, columns int) []xlsx.Cell {
	row := make([]xlsx.Cell, columns)
	row[0] = xlsx.StyledText(title, xlsx.StyleSection)
	return row
}

func headerRow(labels ...string) []xlsx.Cell {
	row := make([]xlsx.Cell, len(labels))
	for i, label := range labels {
		row[i] = xlsx.StyledText(label, xlsx.StyleHeader)
	}
	return row
}

func latestPVETasks(tasks []domain.PVEBackupTask) []domain.PVEBackupTask {
	latest := make(map[int64]domain.PVEBackupTask, len(tasks))
	for _, task := range tasks {
		current, exists := latest[task.VMID]
		if !exists || task.StartTime > current.StartTime {
			latest[task.VMID] = task
		}
	}
	result := make([]domain.PVEBackupTask, 0, len(latest))
	for _, task := range latest {
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].VMID < result[j].VMID })
	return result
}

func backupStatusStyle(status string) int {
	if domain.PVEBackupStatusOK(status) {
		return xlsx.StyleOK
	}
	if domain.PVEBackupStatusWarning(status) {
		return xlsx.StyleWarning
	}
	return xlsx.StyleError
}

func diskHealthStyle(health string) int {
	value := strings.ToLower(strings.TrimSpace(health))
	if value == "" || value == "unknown" || value == "desconocido" {
		return xlsx.StyleWarning
	}
	if strings.Contains(value, "ok") || strings.Contains(value, "healthy") || strings.Contains(value, "saludable") {
		return xlsx.StyleOK
	}
	return xlsx.StyleError
}

func pbsTaskTypeLabel(taskType string) string {
	if taskType == "gc" {
		return "Garbage collection"
	}
	return "Sincronización"
}

func tableFilter(headerRow, totalRows, columns int) string {
	if totalRows <= headerRow {
		return ""
	}
	return fmt.Sprintf("A%d:%s%d", headerRow, spreadsheetColumn(columns), totalRows)
}

func spreadsheetColumn(column int) string {
	result := ""
	for column > 0 {
		column--
		result = string(rune('A'+column%26)) + result
		column /= 26
	}
	return result
}

func unixTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func reportNow(h *WebH) time.Time {
	now := time.Now()
	if h != nil && h.tmpl != nil && h.tmpl.loc != nil {
		return now.In(h.tmpl.loc)
	}
	return now
}

func reportIsFromToday(reportedAt, generatedAt time.Time) bool {
	if reportedAt.IsZero() {
		return false
	}
	reportedAt = reportedAt.In(generatedAt.Location())
	return reportedAt.Year() == generatedAt.Year() && reportedAt.YearDay() == generatedAt.YearDay()
}

func reportFreshnessCell(current bool) xlsx.Cell {
	if current {
		return xlsx.StyledText("Actual", xlsx.StyleOK)
	}
	return xlsx.StyledText("Último disponible", xlsx.StyleWarning)
}

func serveXLSX(w http.ResponseWriter, filename string, workbook xlsx.Workbook) {
	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if err := workbook.Write(w); err != nil {
		http.Error(w, "error generando el informe", http.StatusInternalServerError)
	}
}

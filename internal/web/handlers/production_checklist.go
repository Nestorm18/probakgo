package webhandlers

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/netutil"
)

type productionChecklistView struct {
	Items    []productionChecklistItem
	OK       int
	Warnings int
	Bad      int
	Ready    bool
}

type productionChecklistItem struct {
	Title       string
	Detail      string
	Status      string
	CSSClass    string
	Icon        string
	ActionURL   string
	ActionPost  string
	ActionLabel string
}

func (h *WebH) buildProductionChecklist(r *http.Request, cfg *domain.EmailConfig) productionChecklistView {
	view := productionChecklistView{}
	add := func(item productionChecklistItem) {
		view.Items = append(view.Items, item)
		switch item.CSSClass {
		case "ok":
			view.OK++
		case "bad":
			view.Bad++
		case "warn":
			view.Warnings++
		}
	}

	scheme := netutil.RequestScheme(r)
	host := netutil.HostFromRequest(r)
	publicHost := netutil.HostLooksPublic(host)
	publicURLScheme := publicURLScheme(cfg)
	switch {
	case scheme == "https":
		add(checklistOK("HTTPS detectado", "El panel se esta usando por HTTPS.", "bi-shield-check"))
	case publicURLScheme == "https":
		add(checklistOK("HTTPS configurado", "La URL publica configurada usa HTTPS.", "bi-shield-check"))
	case publicHost:
		add(checklistBad("HTTPS no detectado", "El acceso actual parece publico y usa HTTP.", "bi-exclamation-triangle", "/settings/system", "Configurar URL/HTTPS"))
	default:
		add(checklistWarn("HTTPS no detectado", "HTTP solo deberia usarse en LAN o por NetBird/VPN.", "bi-shield-exclamation", "/settings/system", "Revisar acceso"))
	}

	if h.tmpl != nil && h.tmpl.secure {
		add(checklistOK("SESSION_SECURE=true", "La cookie de sesion se marca como segura.", "bi-cookie"))
	} else {
		item := checklistBad("SESSION_SECURE=false", "Activalo si hay HTTPS delante para proteger la cookie.", "bi-cookie", "/settings/system", "Ver sistema")
		if scheme == "https" {
			item.Detail = "HTTPS detectado. Puedes guardar SESSION_SECURE=true en .env y reiniciar el servicio."
			item.ActionURL = ""
			item.ActionPost = "/settings/system/session-secure"
			item.ActionLabel = "Activar"
		}
		add(item)
	}

	if cfg != nil && cfg.EnforceTOTPNonReaders {
		add(checklistOK("2FA obligatorio", "Administradores y editores deben activar 2FA.", "bi-person-lock"))
	} else {
		add(checklistBad("2FA no obligatorio", "Activa 2FA obligatorio para usuarios admin/editor.", "bi-person-lock", "/settings/system", "Activar 2FA"))
	}

	if cfg != nil && cfg.SensitiveActionsRequireTOTP {
		add(checklistOK("2FA en operaciones delicadas", "Cambios sensibles requieren 2FA activo.", "bi-fingerprint"))
	} else {
		add(checklistBad("Operaciones delicadas sin 2FA", "API keys, usuarios y configuracion deberian requerir 2FA.", "bi-fingerprint", "/settings/system", "Activar proteccion"))
	}

	missingTOTP := h.countPrivilegedUsersWithoutTOTP(r)
	if missingTOTP == 0 {
		add(checklistOK("Cuentas privilegiadas", "Todos los admin/editor activos tienen 2FA.", "bi-people"))
	} else {
		add(checklistBad("Cuentas sin 2FA", fmt.Sprintf("%d usuario(s) admin/editor activo(s) no tienen 2FA.", missingTOTP), "bi-people", "/users", "Revisar usuarios"))
	}

	if cfg != nil && cfg.PublicAPIURL != "" {
		if publicURLScheme == "https" {
			add(checklistOK("URL publica configurada", cfg.PublicAPIURL, "bi-globe2"))
		} else {
			add(checklistWarn("URL publica sin HTTPS", cfg.PublicAPIURL, "bi-globe2", "/settings/system", "Revisar URL"))
		}
	} else {
		add(checklistWarn("URL publica no configurada", "Los instaladores usaran la URL actual del navegador.", "bi-globe2", "/settings/system", "Configurar URL"))
	}

	version := ""
	if h.tmpl != nil {
		version = h.tmpl.version
	}
	add(checklistOK("Version cargada", "v"+version, "bi-tag"))

	add(h.emailChecklistItem(r, cfg))

	if cfg != nil && cfg.RetentionEnabled && cfg.RetentionMonths > 0 {
		add(checklistOK("Retencion activa", fmt.Sprintf("Reportes historicos: %d mes(es).", cfg.RetentionMonths), "bi-hourglass-split"))
	} else {
		add(checklistWarn("Retencion desactivada", "La base de datos conservara reportes historicos sin limite.", "bi-hourglass-split", "/settings/maintenance", "Configurar retencion"))
	}

	view.Ready = view.Bad == 0
	return view
}

func (h *WebH) countPrivilegedUsersWithoutTOTP(r *http.Request) int {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		return 1
	}
	missing := 0
	for _, u := range users {
		if !u.IsActive || u.Role == "reader" {
			continue
		}
		if !u.TOTPEnabled {
			missing++
		}
	}
	return missing
}

func (h *WebH) emailChecklistItem(r *http.Request, cfg *domain.EmailConfig) productionChecklistItem {
	if cfg == nil || !cfg.IsEnabled {
		return checklistWarn("Email desactivado", "No se enviaran informes ni avisos por correo.", "bi-envelope-exclamation", "/settings/email", "Configurar email")
	}
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" || cfg.SMTPPass == "" || cfg.Recipients == "" {
		return checklistBad("Email incompleto", "Faltan servidor SMTP, credenciales o destinatarios.", "bi-envelope-x", "/settings/email", "Completar email")
	}
	status, err := h.store.GetEmailDeliveryStatus(r.Context())
	if err != nil {
		return checklistWarn("Email sin estado", "No se pudo leer el estado del ultimo envio.", "bi-envelope-exclamation", "/settings/email", "Probar email")
	}
	if status == nil || status.LastAttemptAt == nil {
		return checklistWarn("Email sin prueba registrada", "Envia un email de prueba para confirmar la configuracion.", "bi-envelope-exclamation", "/settings/email", "Probar email")
	}
	if status.LastError != "" && (status.LastSuccessAt == nil || status.LastAttemptAt.After(*status.LastSuccessAt)) {
		return checklistBad("Ultimo email fallo", status.LastError, "bi-envelope-x", "/settings/email", "Revisar email")
	}
	if status.LastSuccessAt == nil {
		return checklistWarn("Email sin OK registrado", "Todavia no hay un envio correcto registrado.", "bi-envelope-exclamation", "/settings/email", "Probar email")
	}
	age := time.Since(*status.LastSuccessAt)
	if age > 48*time.Hour {
		return checklistWarn("Ultimo email OK antiguo", "Ultimo envio correcto: "+h.formatChecklistTime(*status.LastSuccessAt), "bi-envelope-check", "/settings/email", "Probar email")
	}
	return checklistOK("Ultimo email OK", "Ultimo envio correcto: "+h.formatChecklistTime(*status.LastSuccessAt), "bi-envelope-check")
}

func publicURLScheme(cfg *domain.EmailConfig) string {
	if cfg == nil || cfg.PublicAPIURL == "" {
		return ""
	}
	u, err := url.Parse(cfg.PublicAPIURL)
	if err != nil {
		return ""
	}
	return u.Scheme
}

func (h *WebH) formatChecklistTime(t time.Time) string {
	if h.tmpl != nil && h.tmpl.loc != nil {
		t = t.In(h.tmpl.loc)
	}
	return t.Format("02/01/2006 15:04")
}

func checklistOK(title, detail, icon string) productionChecklistItem {
	return productionChecklistItem{Title: title, Detail: detail, Status: "OK", CSSClass: "ok", Icon: icon}
}

func checklistWarn(title, detail, icon, actionURL, actionLabel string) productionChecklistItem {
	return productionChecklistItem{Title: title, Detail: detail, Status: "Revisar", CSSClass: "warn", Icon: icon, ActionURL: actionURL, ActionLabel: actionLabel}
}

func checklistBad(title, detail, icon, actionURL, actionLabel string) productionChecklistItem {
	return productionChecklistItem{Title: title, Detail: detail, Status: "Accion", CSSClass: "bad", Icon: icon, ActionURL: actionURL, ActionLabel: actionLabel}
}

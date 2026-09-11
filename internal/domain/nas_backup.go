package domain

type NASBackupConfig struct {
	Enabled              bool
	Host                 string
	Port                 int
	Username             string
	Password             string
	Directory            string
	SendTime             string
	LastScheduledAttempt string
	LastAttempt          string
	LastSuccess          string
	LastError            string
}

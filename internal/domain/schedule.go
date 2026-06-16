package domain

import "time"

// VMScheduledForDay reports whether c has a backup scheduled on the given weekday.
func VMScheduledForDay(c VMBackupConfig, day time.Weekday) bool {
	switch day {
	case time.Monday:
		return c.Monday
	case time.Tuesday:
		return c.Tuesday
	case time.Wednesday:
		return c.Wednesday
	case time.Thursday:
		return c.Thursday
	case time.Friday:
		return c.Friday
	case time.Saturday:
		return c.Saturday
	case time.Sunday:
		return c.Sunday
	}
	return false
}

func VMHasAnyScheduledDay(c VMBackupConfig) bool {
	return c.Monday || c.Tuesday || c.Wednesday || c.Thursday || c.Friday || c.Saturday || c.Sunday
}

func HasActiveVMBackupConfigs(configs []VMBackupConfig) bool {
	for _, c := range configs {
		if !c.IsExcluded && VMHasAnyScheduledDay(c) {
			return true
		}
	}
	return false
}

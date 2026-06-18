package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const logRetentionDays = 7

func setupLogging() func() {
	log.SetFlags(log.LstdFlags)
	if err := os.MkdirAll(installDir(), 0755); err != nil {
		log.SetOutput(os.Stdout)
		return func() {}
	}
	if err := prepareLogFile(time.Now()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "WARN: could not rotate logs: %v\n", err)
	}
	f, err := os.OpenFile(logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.SetOutput(os.Stdout)
		return func() {}
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	return func() {
		_ = f.Close()
	}
}

func prepareLogFile(now time.Time) error {
	if err := rotateCurrentLog(now); err != nil {
		return err
	}
	return pruneOldLogs(now)
}

func rotateCurrentLog(now time.Time) error {
	path := logPath()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if sameLogDay(info.ModTime(), now) {
		return nil
	}
	archive := filepath.Join(installDir(), "probakgo-windows-client-"+info.ModTime().Format("2006-01-02")+".log")
	if err := appendFile(path, archive); err != nil {
		return err
	}
	return os.Remove(path)
}

func appendFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func pruneOldLogs(now time.Time) error {
	matches, err := filepath.Glob(filepath.Join(installDir(), "probakgo-windows-client-*.log"))
	if err != nil {
		return err
	}
	cutoff := dayStart(now).AddDate(0, 0, -(logRetentionDays - 1))
	for _, path := range matches {
		day, ok := archiveLogDay(path, now.Location())
		if !ok {
			continue
		}
		if day.Before(cutoff) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func archiveLogDay(path string, loc *time.Location) (time.Time, bool) {
	name := filepath.Base(path)
	dateText := strings.TrimSuffix(strings.TrimPrefix(name, "probakgo-windows-client-"), ".log")
	day, err := time.ParseInLocation("2006-01-02", dateText, loc)
	return day, err == nil
}

func sameLogDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.In(a.Location()).Date()
	return ay == by && am == bm && ad == bd
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

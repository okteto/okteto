package log

import syLogger "github.com/syncthing/syncthing/lib/logger"

// GetLog returns the command logger
func GetLog() *logger {
	return log
}

func (l *logger) Debugln(vals ...interface{}) {
	Debug(vals...)
}
func (l *logger) Debugf(format string, vals ...interface{}) {
	Debugf(format, vals...)
}

func (l *logger) Verboseln(vals ...interface{}) {
	Debug(vals...)
}

func (l *logger) Verbosef(format string, vals ...interface{}) {
	Debugf(format, vals...)
}

func (l *logger) Infoln(vals ...interface{}) {
	Info(vals...)
}

func (l *logger) Infof(format string, vals ...interface{}) {
	Infof(format, vals...)
}

func (l *logger) Warnln(vals ...interface{}) {
	Warn(vals...)
}

func (l *logger) Warnf(format string, vals ...interface{}) {
	Warnf(format, vals...)
}

// no-ops
func (l *logger) AddHandler(level syLogger.LogLevel, h syLogger.MessageHandler) {}
func (l *logger) SetFlags(flag int)                                             {}
func (l *logger) SetPrefix(prefix string)                                       {}
func (l *logger) ShouldDebug(facility string) bool                              { return false }
func (l *logger) SetDebug(facility string, enabled bool)                        {}
func (l *logger) Facilities() map[string]string                                 { return make(map[string]string) }
func (l *logger) FacilityDebugging() []string                                   { return make([]string, 0) }
func (l *logger) NewFacility(facility, description string) syLogger.Logger      { return GetLog() }

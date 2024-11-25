package log

import (
	"io"
	stdLog "log"
)

var (
	component string
	isDebug   bool
)

func SetComponentName(name string) {
	component = name
}

func SetIsDebug(debug bool) {
	isDebug = debug
}

func Debugf(template string, args ...interface{}) {
	if isDebug {
		stdLog.Printf("DEBUG["+component+"]: "+template, args...)
	}
}

func Infof(template string, args ...interface{}) {
	stdLog.Printf("INFO["+component+"]: "+template, args...)
}

func Printf(template string, args ...interface{}) {
	stdLog.Printf("INFO["+component+"]: "+template, args...)
}

func Warnf(template string, args ...interface{}) {
	stdLog.Printf("WARN["+component+"]: "+template, args...)
}

func Errorf(template string, args ...interface{}) {
	stdLog.Printf("ERR["+component+"]: "+template, args...)
}

func Fatalf(template string, args ...interface{}) {
	stdLog.Fatalf(template, args...)
}

func Fatal(err string) {
	stdLog.Fatal(err)
}

type taggedWritter struct {
	tag    string
	logger *stdLog.Logger
}

func NewTaggedWriter(writer io.Writer, tag string) io.Writer {
	return &taggedWritter{tag: tag, logger: stdLog.New(writer, "", stdLog.LstdFlags)}
}

func (w *taggedWritter) Write(p []byte) (n int, err error) {
	w.logger.Printf("LOG[%s]: %s", w.tag, string(p))
	return len(p), nil
}

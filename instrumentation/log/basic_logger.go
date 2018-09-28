package log

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type BasicLogger interface {
	Log(level string, message string, params ...*Field)
	LogFailedExpectation(message string, expected *Field, actual *Field, params ...*Field)
	Info(message string, params ...*Field)
	Error(message string, params ...*Field)
	Metric(name string, params ...*Field)
	WithTag(params ...*Field) BasicLogger
	Meter(name string, params ...*Field) BasicMeter
	Tags() []*Field
	WithOutput(writer ...Output) BasicLogger
	WithFilter(filter Filter) BasicLogger
}

type basicLogger struct {
	outputs               []Output
	tags                  []*Field
	nestingLevel          int
	sourceRootPrefixIndex int
	filters               []Filter
}

const (
	NoType = iota
	ErrorType
	NodeType
	ServiceType
	StringType
	IntType
	UintType
	BytesType
	FloatType
	FunctionType
	SourceType
	StringArrayType
)

func GetLogger(params ...*Field) BasicLogger {
	logger := &basicLogger{
		tags:         params,
		nestingLevel: 4,
		outputs:      []Output{&basicOutput{output: os.Stdout, formatter: NewJsonFormatter()}},
	}

	fpcs := make([]uintptr, 1)
	n := runtime.Callers(logger.nestingLevel, fpcs)
	if n != 0 {
		fun := runtime.FuncForPC(fpcs[0] - 1)
		if fun != nil {
			file, _ := fun.FileLine(fpcs[0] - 1)
			if l := strings.Index(file, "orbs-network-go/"); l > -1 {
				logger.sourceRootPrefixIndex = l + len("orbs-network-go/")
			}
		}
	}

	return logger
}

func (b *basicLogger) getCaller(level int) (function string, source string) {
	fpcs := make([]uintptr, 1)

	// skip levels to get to the caller of logger function
	n := runtime.Callers(level, fpcs)
	if n == 0 {
		return "n/a", "n/a"
	}

	fun := runtime.FuncForPC(fpcs[0] - 1)
	if fun == nil {
		return "n/a", "n/a"
	}

	file, line := fun.FileLine(fpcs[0] - 1)
	fName := fun.Name()
	lastSlashOfName := strings.LastIndex(fName, "/")
	if lastSlashOfName > 0 {
		fName = fName[lastSlashOfName+1:]
	}
	return fName, fmt.Sprintf("%s:%d", file[b.sourceRootPrefixIndex:], line)
}

func (b *basicLogger) Tags() []*Field {
	return b.tags
}

func (b *basicLogger) WithTag(params ...*Field) BasicLogger {
	prefixes := append(b.tags, params...)
	return &basicLogger{tags: prefixes, nestingLevel: b.nestingLevel, outputs: b.outputs, sourceRootPrefixIndex: b.sourceRootPrefixIndex, filters: b.filters}
}

func (b *basicLogger) Metric(metric string, params ...*Field) {
	metricParams := append(params, String("metric", metric))
	b.Log("metric", "Metric recorded", metricParams...)
}

func (b *basicLogger) Log(level string, message string, params ...*Field) {
	function, source := b.getCaller(b.nestingLevel)

	enrichmentParams := []*Field{
		Function(function),
		Source(source),
	}

	enrichmentParams = append(enrichmentParams, b.tags...)
	enrichmentParams = append(enrichmentParams, params...)

	for _, f := range b.filters {
		if !f.Allows(level, message, enrichmentParams) {
			return
		}
	}

	for _, output := range b.outputs {
		logLine := output.Formatter().FormatRow(level, message, enrichmentParams...)
		fmt.Fprintln(output.Output(), logLine)
	}
}

func (b *basicLogger) Info(message string, params ...*Field) {
	b.Log("info", message, params...)
}

func (b *basicLogger) Error(message string, params ...*Field) {
	b.Log("error", message, params...)
}

func (b *basicLogger) LogFailedExpectation(message string, expected *Field, actual *Field, params ...*Field) {
	actual.Key = "actual-" + actual.Key
	expected.Key = "expected-" + expected.Key
	newParams := append(params, expected, actual)
	b.Log("expectation", message, newParams...)
}

func (b *basicLogger) Meter(name string, params ...*Field) BasicMeter {
	meterLogger := &basicLogger{nestingLevel: 5, tags: b.tags, outputs: b.outputs}
	return &basicMeter{name: name, start: time.Now().UnixNano(), logger: meterLogger, params: params}
}

func (b *basicLogger) WithOutput(writers ...Output) BasicLogger {
	b.outputs = writers
	return b
}

func (b *basicLogger) WithFilter(filter Filter) BasicLogger {
	b.filters = append(b.filters, filter) // this is not thread safe, I know
	return b
}

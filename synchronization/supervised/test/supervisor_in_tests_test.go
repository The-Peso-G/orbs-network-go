package test

import "testing"

var expectedLogsOnPanic = []string{
	Failed, BeforeLoggerCreated, LoggedWithLogger, BeforeCallPanic, PanicOhNo,
}
var unexpectedLogsOnPanic = []string{
	Passed, AfterCallPanic, MustNotShow,
}

var expectedLogsOnLogError = []string{
	Failed, BeforeLoggerCreated, LoggedWithLogger, BeforeLoggerError, ErrorWithLogger, AfterLoggerError, MustShow,
}
var unexpectedLogsOnLogError = []string{
	Passed, MustNotShow,
}

func Test_Panics(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnPanic, unexpectedLogsOnPanic)
}

func Test_LogsError(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnLogError, unexpectedLogsOnLogError)
}

func TestGoOnce_Panics(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnPanic, unexpectedLogsOnPanic)
}

func TestGoOnce_LogsError(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnLogError, unexpectedLogsOnLogError)
}

func TestTRun_Panics(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnPanic, unexpectedLogsOnPanic)
}

func TestTRun_LogsError(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnLogError, unexpectedLogsOnLogError)
}

func TestTRun_GoOnce_Panics(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnPanic, unexpectedLogsOnPanic)
}

func TestTRun_GoOnce_LogsError(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnLogError, unexpectedLogsOnLogError)
}

func TestTRun_GoOnce_PanicsAfterSubTestPasses(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnPanic, unexpectedLogsOnPanic)
}

func TestTRun_GoOnce_LogsErrorAfterSubTestPasses(t *testing.T) {
	executeGoTestRunner(t, expectedLogsOnLogError, unexpectedLogsOnLogError)
}

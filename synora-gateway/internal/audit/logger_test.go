package audit

import (
	"testing"
)

func TestNewLogDispatcher(t *testing.T) {
	// We can't easily test startWorker because it calls clickhouse.GetConn()
	// and ClickHouse might not be initialized.
	// But we can test that NewLogDispatcher creates the struct.
	
	d := NewLogDispatcher(10)
	if d == nil {
		t.Errorf("NewLogDispatcher returned nil")
	}
	if d.logChan == nil {
		t.Errorf("logChan is nil")
	}
}

func TestLogDispatcher_Log(t *testing.T) {
	d := &LogDispatcher{
		logChan: make(chan *CallLog, 1),
	}
	
	entry := &CallLog{RequestID: "test"}
	d.Log(entry)
	
	select {
	case received := <-d.logChan:
		if received.RequestID != "test" {
			t.Errorf("Expected request ID test, got %s", received.RequestID)
		}
	default:
		t.Errorf("Log entry was not sent to channel")
	}
	
	// Test full buffer
	d.Log(entry) // fill buffer
	d.Log(entry) // this should drop log (select default)
}

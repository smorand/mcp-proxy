package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Span represents a single trace span in JSONL format.
type Span struct {
	TraceID     string         `json:"trace_id"`
	SpanID      string         `json:"span_id"`
	Name        string         `json:"name"`
	ServiceName string         `json:"service_name"`
	StartTime   string         `json:"start_time"`
	EndTime     string         `json:"end_time"`
	Status      string         `json:"status"`
	Attributes  map[string]any `json:"attributes"`
}

// FileExporter writes trace spans as JSONL to a file.
type FileExporter struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileExporter creates a JSONL file exporter at the given path with 0600 permissions.
func NewFileExporter(path string) (*FileExporter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open trace file %s: %w", path, err)
	}
	// Ensure permissions are exactly 0600 regardless of umask
	if err := os.Chmod(path, 0600); err != nil {
		f.Close()
		return nil, fmt.Errorf("set trace file permissions %s: %w", path, err)
	}
	return &FileExporter{file: f}, nil
}

// Export writes a span as a single JSONL line.
func (e *FileExporter) Export(span *Span) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	data, err := json.Marshal(span)
	if err != nil {
		return fmt.Errorf("marshal span: %w", err)
	}

	data = append(data, '\n')
	_, err = e.file.Write(data)
	return err
}

// Close closes the underlying file.
func (e *FileExporter) Close() error {
	return e.file.Close()
}

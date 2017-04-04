package remotemachine

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	boshssh "github.com/cloudfoundry/bosh-cli/ssh"
)

type MemWriter struct {
	instances map[string]*memWriterInstance
	lock      *sync.RWMutex
}

func NewMemWriter() *MemWriter {
	return &MemWriter{
		instances: map[string]*memWriterInstance{},
		lock:      &sync.RWMutex{},
	}
}

func (w *MemWriter) ForInstance(jobName, indexOrID string) boshssh.InstanceWriter {
	w.lock.Lock()
	defer w.lock.Unlock()

	instance := &memWriterInstance{
		jobName:   jobName,
		indexOrID: indexOrID,

		stdout: bytes.NewBufferString(""),
		stderr: bytes.NewBufferString(""),
	}

	id := fmt.Sprintf("%s/%s", jobName, indexOrID)
	w.instances[id] = instance
	return instance
}

func (w *MemWriter) Flush() {}

func (w *MemWriter) ResultsForInstance(jobName, indexOrID string) *memWriterInstance {
	w.lock.RLock()
	defer w.lock.RUnlock()

	id := fmt.Sprintf("%s/%s", jobName, indexOrID)
	if instance, ok := w.instances[id]; ok {
		return instance
	}
	return nil
}

type memWriterInstance struct {
	jobName   string
	indexOrID string

	stdout *bytes.Buffer
	stderr *bytes.Buffer

	exitStatus int
	err        error
}

func (w *memWriterInstance) Stdout() io.Writer { return w.stdout }
func (w *memWriterInstance) Stderr() io.Writer { return w.stderr }
func (w *memWriterInstance) End(exitStatus int, err error) {
	w.exitStatus = exitStatus
	w.err = err
}

func (w *memWriterInstance) JobName() string         { return w.jobName }
func (w *memWriterInstance) IndexOrID() string       { return w.indexOrID }
func (w *memWriterInstance) StdoutReader() io.Reader { return w.stdout }
func (w *memWriterInstance) StdoutString() string    { return w.stdout.String() }
func (w *memWriterInstance) StderrReader() io.Reader { return w.stderr }
func (w *memWriterInstance) StderrString() string    { return w.stderr.String() }
func (w *memWriterInstance) ExitStatus() int         { return w.exitStatus }
func (w *memWriterInstance) Error() error            { return w.err }

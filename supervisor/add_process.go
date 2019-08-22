package supervisor

import (
	"os"
	"time"

	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/specs"
)

// AddProcessTask holds everything necessary to add a process to a
// container
type AddProcessTask struct {
	baseTask
	ID            string
	PID           string
	Stdout        string
	Stderr        string
	Stdin         string
	ProcessSpec   *specs.ProcessSpec
	StartResponse chan StartResponse
}

type execTask struct {
	t  *AddProcessTask
	ci *containerInfo
}

func (s *Supervisor) addProcess(t *AddProcessTask) error {
	ci, ok := s.containers[t.ID]
	if !ok {
		return ErrContainerNotFound
	}
	s.execTasks <- &execTask{
		t:  t,
		ci: ci,
	}
	return errDeferredResponse
}

func (s *Supervisor) execWorker(id int) {
	for p := range s.execTasks {
		var (
			start = time.Now()
			ci    = p.ci
			t     = p.t
		)

		process, err := ci.container.Exec(t.Ctx(), t.PID, *t.ProcessSpec, runtime.NewStdio(t.Stdin, t.Stdout, t.Stderr))
		if err != nil {
			t.ErrorCh() <- err
			continue
		}
		s.newExecSyncChannel(t.ID, t.PID)

		if err := s.monitorProcess(process); err != nil {
			s.deleteExecSyncChannel(t.ID, t.PID)
			// Kill process
			process.Signal(os.Kill)
			ci.container.RemoveProcess(t.PID)
			t.ErrorCh() <- err
			continue
		}
		ExecProcessTimer.UpdateSince(start)

		t.ErrorCh() <- nil
		t.StartResponse <- StartResponse{ExecPid: process.SystemPid()}

		s.notifySubscribers(Event{
			Timestamp: time.Now(),
			Type:      StateStartProcess,
			PID:       t.PID,
			ID:        t.ID,
		})
	}
}

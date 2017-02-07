package supervisor

import (
	"sync"

	api "github.com/docker/containerd/api/execution"
	"github.com/docker/containerd/api/shim"
	"golang.org/x/net/context"
)

func newCollector(ctx context.Context) *collector {
	c := &collector{
		context: ctx,
		ch:      make(chan *shim.Event, 2048),
		streams: make(map[*stream]struct{}),
	}
	go c.waitDone()
	return c
}

type stream struct {
	eCh    chan error
	stream api.ExecutionService_EventsServer
}

type collector struct {
	mu sync.Mutex
	wg sync.WaitGroup

	context context.Context
	ch      chan *shim.Event
	streams map[*stream]struct{}
}

func (c *collector) Request() *shim.EventsRequest {
	return &shim.EventsRequest{}
}

func (c *collector) Collect(events shim.Shim_EventsClient) error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			e, err := events.Recv()
			if err != nil {
				// TODO: log errors
				// what is the grpc error message when the connection is closed?
				return
			}
			c.ch <- e
		}
	}()
	return nil
}

func (c *collector) Publish(s api.ExecutionService_EventsServer) (err error) {
	sr := &stream{
		stream: s,
		eCh:    make(chan error, 1),
	}
	c.mu.Lock()
	c.streams[sr] = struct{}{}
	c.mu.Unlock()
	if serr := <-sr.eCh; serr != nil {
		err = serr
	}
	c.mu.Lock()
	delete(c.streams, sr)
	c.mu.Unlock()
	return err
}

func (c *collector) publisher() {
	for e := range c.ch {
		c.mu.Lock()
		for s := range c.streams {
			if err := s.stream.Send(&api.Event{
				ID:         e.ID,
				Type:       api.EventType(int32(e.Type)),
				Pid:        e.Pid,
				ExitStatus: e.ExitStatus,
			}); err != nil {
				s.eCh <- err
			}
		}
		c.mu.Unlock()
	}
}

// waitDone waits for the context to finish, waits for all the goroutines to finish
// collecting grpc events from the shim, and closes the output channel
func (c *collector) waitDone() {
	<-c.context.Done()
	c.wg.Wait()
	close(c.ch)
}

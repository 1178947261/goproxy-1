package msocks

import (
	"io"
	"sync"
)

type Window struct {
	c       *sync.Cond
	mu      *sync.Mutex
	closed  bool
	win     uint32
	max     uint32
	sendmsg bool
}

func NewWindow(init uint32) (w *Window) {
	var mu sync.Mutex
	w = &Window{
		c:       sync.NewCond(&mu),
		mu:      &mu,
		win:     init,
		max:     init,
		sendmsg: true,
	}
	return
}

func (w *Window) Close() (err error) {
	w.closed = true
	w.c.Broadcast()
	return
}

func (w *Window) Acquire() (n uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for {
		switch {
		case w.closed:
			return
		case w.win == 0:
			w.c.Wait()
			continue
		default:
			n = 1
		}
		w.win -= n
		return
	}
}

func (w *Window) Release(num uint32) (n uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.win += num
	if w.win > w.max {
		w.win = w.max
	}
	n = w.win
	w.c.Broadcast()
	return
}

// write in seq,
type SeqWriter struct {
	Window
	closed bool
	lock   sync.Mutex
	sess   *Session
}

func NewSeqWriter(sess *Session) (sw *SeqWriter) {
	return &SeqWriter{
		Window: *NewWindow(WIN_SIZE),
		sess:   sess,
	}
}

func (sw *SeqWriter) Ack(streamid uint16, n int32) (err error) {
	b := NewFrameOneInt(MSG_ACK, streamid, uint32(n))
	err = sw.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	return
}

func (sw *SeqWriter) Data(streamid uint16, data []byte) (err error) {
	if len(data) == 0 {
		return
	}
	// check for window
	if sw.Acquire() == 0 {
		// that mean closed
		return io.EOF
	}
	b, err := NewFrameData(streamid, data)
	if err != nil {
		logger.Err(err)
		return
	}
	err = sw.WriteStream(streamid, b)
	if err == io.EOF {
		err = nil
	}
	return
}

func (sw *SeqWriter) WriteStream(streamid uint16, b []byte) (err error) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	if sw.closed {
		return io.EOF
	}
	err = sw.sess.WriteStream(streamid, b)
	if err == io.EOF {
		sw.closed = true
	}
	return
}

func (sw *SeqWriter) DontSend() {
	sw.sendmsg = false
}

// TODO: remove closed?
func (sw *SeqWriter) Close(streamid uint16) (err error) {
	sw.lock.Lock()
	defer sw.lock.Unlock()
	if sw.closed {
		return io.EOF
	}
	sw.closed = true
	sw.Window.Close()

	if sw.sendmsg {
		// send fin if not closed yet.
		b := NewFrameNoParam(MSG_FIN, streamid)
		err = sw.sess.WriteStream(streamid, b)
		if err == io.EOF {
			err = nil
		}
	}
	return
}

func (sw *SeqWriter) Closed() bool {
	return sw.closed
}

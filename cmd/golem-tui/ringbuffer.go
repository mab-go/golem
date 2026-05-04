package main

// ringBuffer is a fixed-capacity circular buffer of strings.
type ringBuffer struct {
	lines []string
	cap   int
	head  int
	count int
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		lines: make([]string, capacity),
		cap:   capacity,
	}
}

func (r *ringBuffer) Push(line string) {
	r.lines[r.head] = line
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

func (r *ringBuffer) Lines() []string {
	if r.count == 0 {
		return nil
	}
	start := (r.head - r.count + r.cap) % r.cap
	if start < r.head {
		return r.lines[start:r.head]
	}
	result := make([]string, 0, r.count)
	result = append(result, r.lines[start:]...)
	result = append(result, r.lines[:r.head]...)
	return result
}

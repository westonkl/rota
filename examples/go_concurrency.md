---
deck: "Go Concurrency"
---

# Go Concurrency Flashcards

Q: What is a Goroutine in Go? #go #concurrency
A: A goroutine is a lightweight thread of execution managed independently by the Go runtime scheduler rather than the OS kernel. It starts with a small stack (typically 2KB) that grows and shrinks dynamically.

---

Q: What happens when you read from a closed Go channel? #channels #go
A: Reading from a closed channel immediately returns the zero-value of the channel's element type and `false` as the second ok-boolean:
```go
val, ok := <-ch
// ok is false if ch is closed and empty
```

---

Q: What happens when you send to a closed Go channel? #channels #go
A: Sending to a closed channel causes a **panic** at runtime.

---

C: In Go, communication over an unbuffered channel is [synchronous] and blocks until both the sender and receiver are ready. #channels

---

C: A `sync.RWMutex` allows multiple [readers] to hold the lock simultaneously, but only one [writer] at a time. #sync

---

Q: How does `sync.Once` ensure an initialization function runs only once? #sync
A: `sync.Once` uses an atomic check on an internal `done` uint32 flag followed by a mutex fallback (fast-path atomic load + slow-path lock) to guarantee atomic, single execution even across concurrent goroutines.

---

C: In Go select statements, if multiple cases are simultaneously ready, Go picks one [pseudo-randomly] to prevent starvation. #select

Q: What is the memory model rule for channel send vs receive in Go? #go #concurrency
A: A send on a channel happens before the corresponding receive from that channel completes.

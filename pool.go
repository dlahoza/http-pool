// Copyright 2015 Dmitry Lagoza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package http

import (
	"net"
	"time"
)

// Objects implementing the http workers pool
type Pool struct {
	total, busy   uint
	jobsChan      chan *job
	startQuitChan chan bool
	fullStopChan  chan bool
	busyFreeChan  chan bool
}

type job struct {
	c    *conn
	stop bool
}

// manager handles "start worker", "worker quit", "worker busy or free"
// and "stop all worker" requests
func (p *Pool) manager() {
	for {
		select {
		case c := <-p.startQuitChan:
			if c {
				go p.worker()
				p.total++
			} else {
				p.total--
			}
		case b := <-p.busyFreeChan:
			if b {
				p.busy++
			} else {
				p.busy--
			}
		case <-p.fullStopChan:
			if p.total > 0 {
				p.StopWorker()
				p.fullStopChan <- true
			}
		}
	}
}

// worker waits job then executes it
func (p *Pool) worker() {
	for {
		job := <-p.jobsChan
		if job.stop {
			p.startQuitChan <- false
			return
		}
		p.busyFreeChan <- true
		job.c.serve()
		p.busyFreeChan <- false
	}
}

// GetTotal get total count of workers in the pool
func (p *Pool) GetTotal() uint {
	return p.total
}

// GetFree get count of free workers in the pool
func (p *Pool) GetFree() uint {
	return p.total - p.busy
}

// GetBusy get count of busy workers in the pool
func (p *Pool) GetBusy() uint {
	return p.busy
}

// AddJob adds connection to the job's channel
func (p *Pool) AddJob(c *conn) {
	p.jobsChan <- &job{c: c, stop: false}
}

// StopWorker sends stop signal to a single worker
func (p *Pool) StopWorker() {
	p.jobsChan <- &job{c: nil, stop: true}
}

// StartWorker starts a single worker
func (p *Pool) StartWorker() {
	p.startQuitChan <- true
}

// StartWorkers starts a multiple workers
func (p *Pool) StartWorkers(workerCount uint) {
	var i uint
	for i = 0; i < workerCount; i++ {
		p.StartWorker()
	}
}

// FullStop sends the stop signal to all workers in pool
func (p *Pool) FullStop() {
	p.fullStopChan <- true
}

// NewPool creates new pool instance, starts the pool manager and workers.
// For empty pool with pool manager only use workerCount with 0 value.
func NewPool(workerCount uint) *Pool {
	pool := &Pool{jobsChan: make(chan *job), startQuitChan: make(chan bool, 1), fullStopChan: make(chan bool, 1), busyFreeChan: make(chan bool)}
	go pool.manager()
	pool.StartWorkers(workerCount)
	return pool
}

// ListenAndServeWithPool listens on the TCP network address srv.Addr and then
// calls ServeWithPool to handle requests on incoming connections in pool.  If
// srv.Addr is blank, ":http" is used.
func (srv *Server) ListenAndServeWithPool(pool *Pool) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return srv.ServeWithPool(tcpKeepAliveListener{ln.(*net.TCPListener)}, pool)
}

// ListenAndServeWithPool listens on the TCP network address addr
// and then calls ServeWithPool with handler to handle requests
// on incoming connections in pool.  Handler is typically nil,
// in which case the DefaultServeMux is used.
//
// A trivial example server is:
//
//      package main
//
//      import (
//              "io"
//              "net/http"
//              "log"
//      )
//
//      // hello world, the web server
//      func HelloServer(w http.ResponseWriter, req *http.Request) {
//              io.WriteString(w, "hello, world!\n")
//      }
//
//      func main() {
//              http.HandleFunc("/hello", HelloServer)
//		p = http.NewPool(1000)
//              err := http.ListenAndServe(":12345", nil, p)
//              if err != nil {
//                      log.Fatal("ListenAndServe: ", err)
//              }
//      }
func ListenAndServeWithPool(addr string, handler Handler, pool *Pool) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServeWithPool(pool)
}

// ServeWithPool accepts incoming connections on the Listener l, sends a job to pool
// for each.  The pool read requests and then call srv.Handler to reply to them.
func (srv *Server) ServeWithPool(l net.Listener, pool *Pool) error {
	defer l.Close()
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		rw, e := l.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				srv.logf("http: Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0
		c, err := srv.newConn(rw)
		if err != nil {
			continue
		}
		c.setState(c.rwc, StateNew) // before Serve can return
		pool.AddJob(c)
	}
}

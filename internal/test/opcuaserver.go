// MIT License

// Copyright (c) 2018-2019 The gopcua authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// https://github.com/gopcua/opcua/blob/affd2bf105fe37786d69cd3607b5f7ed085f8c90/uatest/server.go

package test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/errors"
	"github.com/gopcua/opcua/ua"
)

// Server runs a python test server.
type Server struct {
	// Path is the path to the Python server.
	Path           string
	ServerPKPath   string
	ServerCertPath string
	// Endpoint is the endpoint address which will be set
	// after the server has started.
	Endpoint string

	// Opts contains the client options required to connect to the server.
	// They are valid after the server has been started.
	Opts   []opcua.Option
	cmd    *exec.Cmd
	waitch chan error
}

// NewServer creates a test server and starts it. The function
// panics if the server cannot be started.
func NewServer(path string, certPath ...string) *Server {
	s := &Server{Path: path, waitch: make(chan error)}
	if len(certPath) == 2 {
		s.ServerPKPath = certPath[0]
		s.ServerCertPath = certPath[1]
	}
	if err := s.Run(); err != nil {
		panic(err)
	}
	return s
}

// Run starts the Python-based server
func (s *Server) Run() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	path := filepath.Join(wd, s.Path)

	py, err := exec.LookPath("python3")
	if err != nil {
		// fallback to python and hope it still points to a python3 version.
		// the Windows python3 installer doesn't seem to create a `python3.exe`
		py, err = exec.LookPath("python")
		if err != nil {
			return errors.Errorf("unable to find Python executable")
		}
	}
	if len(s.ServerCertPath) == 0 || len(s.ServerPKPath) == 0 {
		s.cmd = exec.Command(py, path)
	} else {
		fmt.Printf("running %v opcua_server.py %v %v\n", py, s.ServerPKPath, s.ServerCertPath)
		s.cmd = exec.Command(py, path, s.ServerPKPath, s.ServerCertPath)
	}
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	s.Endpoint = Protocol + Address
	s.Opts = []opcua.Option{opcua.SecurityMode(ua.MessageSecurityModeNone)}
	if err := s.cmd.Start(); err != nil {
		return err
	}
	go func() { s.waitch <- s.cmd.Wait() }()

	// wait until endpoint is available
	errch := make(chan error)
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			c, err := net.Dial("tcp", Address)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			c.Close()
			errch <- nil
		}
		errch <- errors.Errorf("timeout")
	}()

	select {
	case err := <-s.waitch:
		return err
	case err := <-errch:
		return err
	}
}

// Close stops the Python-based server
func (s *Server) Close() error {
	if s.cmd == nil {
		return errors.Errorf("not running")
	}
	go func() { s.cmd.Process.Kill() }()
	return <-s.waitch
}

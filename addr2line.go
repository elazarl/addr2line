package addr2line

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strconv"
	"time"
)

type Addr2line struct {
	cmd *exec.Cmd
	r   io.ReadCloser
	w   io.WriteCloser
}

func NewFromCmd(cmd *exec.Cmd) (*Addr2line, error) {
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	ch := make(chan error)
	go func() {
		if err := cmd.Start(); err != nil {
			ch <- fmt.Errorf("Cannot start addr2line: %s", err)
		}
		all, _ := ioutil.ReadAll(stderr)
		err := cmd.Wait()
		ch <- fmt.Errorf("addr2line exited unexpectedly: %s. Stderr: %q", err, string(all))
	}()

	select {
	case <-time.After(20 * time.Millisecond):
	case err := <-ch:
		return nil, err
	}

	if err := stderr.Close(); err != nil {
		panic("stderr pipe cannot just refuse to close: " + err.Error())
	}
	return &Addr2line{cmd, r, w}, nil
}

func New(elf string) (*Addr2line, error) {
	return NewFromCmd(exec.Command("addr2line", "-fie", elf))
}

type Result struct {
	Function string
	File     string
	Line     int
}

func (a *Addr2line) ResolveString(addr string) ([]Result, error) {
	if _, err := fmt.Fprintf(a.w, "%s\n", addr); err != nil {
		return nil, err
	}
	const _POSIX_PIPE_BUF = 512
	buf := make([]byte, _POSIX_PIPE_BUF)
	// binutil addr2line fflush after writing to pipe. Hopefully would be able to read it atomically
	n, err := a.r.Read(buf)
	if err != nil {
		return nil, err
	}
	if buf[n-1] != '\n' {
		return nil, fmt.Errorf("malformed output, not ending with newline: %q", string(buf[:n]))
	}
	buf = buf[:n-1] // remove last newline
	if bytes.Equal(buf, []byte("??\n??:0")) {
		return nil, nil
	}
	lines := bytes.Split(buf, []byte{'\n'})
	results := []Result{}
	for i := 0; i < len(lines); i += 2 {
		j := bytes.LastIndex(lines[i+1], []byte{':'})
		if j < 0 {
			return nil, fmt.Errorf("cannot find ':' in file name: %s", string(lines[i+1]))
		}
		file := lines[i+1][:j]
		l := lines[i+1][j+1:]
		line, err := strconv.Atoi(string(l))
		if err != nil {
			return nil, fmt.Errorf("cannot convert line number to string: %s", string(lines[i+1]))
		}
		results = append(results, Result{string(lines[i]), string(file), line})
	}
	return results, nil
}

func (a *Addr2line) Resolve(addr uint64) ([]Result, error) {
	return a.ResolveString(fmt.Sprintf("%x", addr))
}

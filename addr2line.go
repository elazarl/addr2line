package addr2line

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strconv"
	"time"
)

type Addr2line struct {
	FilePrefix []byte
	cmd        *exec.Cmd
	r          io.ReadCloser
	w          io.WriteCloser
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
	return &Addr2line{nil, cmd, r, w}, nil
}

func New(elf string) (*Addr2line, error) {
	dirs, err := compDirFromELF(elf)
	if err != nil {
		return nil, err
	}
	a2l, err := NewFromCmd(exec.Command("addr2line", "-fie", elf))
	prefix := lcp(dirs)
	if prefix != "" {
		prefix += "/"
	}
	a2l.FilePrefix = []byte(prefix)
	return a2l, err
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
	const _POSIX_PIPE_BUF = 1024
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
		if bytes.HasPrefix(file, a.FilePrefix) {
			file = file[len(a.FilePrefix):]
		}
		results = append(results, Result{string(lines[i]), string(file), line})
	}
	return results, nil
}

func (a *Addr2line) Resolve(addr uint64) ([]Result, error) {
	return a.ResolveString(fmt.Sprintf("%x", addr))
}

func compDirFromELF(path string) ([]string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	d, err := f.DWARF()
	if err != nil {
		return nil, err
	}
	rv := []string{}
	r := d.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return nil, err
		}
		if e == nil {
			break
		}
		r.SkipChildren()
		if e.Tag != dwarf.TagCompileUnit {
			continue
		}
		for _, field := range e.Field {
			if field.Attr == dwarf.AttrCompDir {
				rv = append(rv, field.Val.(string))
			}
		}
	}
	return rv, nil
}

// from Rosetta Stone:
// lcp finds the longest common prefix of the input strings.
// It compares by bytes instead of runes (Unicode code points).
// It's up to the caller to do Unicode normalization if desired
// (e.g. see golang.org/x/text/unicode/norm).
func lcp(l []string) string {
	// Special cases first
	switch len(l) {
	case 0:
		return ""
	case 1:
		return l[0]
	}
	// LCP of min and max (lexigraphically)
	// is the LCP of the whole set.
	min, max := l[0], l[0]
	for _, s := range l[1:] {
		switch {
		case s < min:
			min = s
		case s > max:
			max = s
		}
	}
	for i := 0; i < len(min) && i < len(max); i++ {
		if min[i] != max[i] {
			return min[:i]
		}
	}
	// In the case where lengths are not equal but all bytes
	// are equal, min is the answer ("foo" < "foobar").
	return min
}

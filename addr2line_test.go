package addr2line

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimpleFile(t *testing.T) {
	d, err := ioutil.TempDir(os.TempDir(), "addr2line.go")
	orPanic(err)
	//defer os.RemoveAll(d)
	ioutil.WriteFile(filepath.Join(d, "a.c"), []byte(`#include <stdio.h>

static inline int g() { return 3; }
int f() { return g(); }

int main() { return f(); }`), 0644)

	elf := filepath.Join(d, "a.out")
	run("gcc", "-ggdb3", filepath.Join(d, "a.c"), "-o", elf)
	nmout, err := exec.Command("nm", "--defined-only", elf).CombinedOutput()
	orPanic(err)
	nmlines := strings.Split(strings.TrimSpace(string(nmout)), "\n")
	addr2line, err := New(elf)
	orPanic(err)
	funcs := make(map[string]string)
	for _, l := range nmlines {
		line := strings.Split(l, " ")
		fmt.Printf("%q\n", l)
		funcs[line[2]] = line[0]
	}
	for _, fn := range []string{"f", "g", "main"} {
		r, err := addr2line.ResolveString(funcs[fn])
		orPanic(err)
		if r[0].Function != fn {
			t.Errorf("Cannot find function %s, addr %s", fn, funcs[fn])
		}
	}
	r, err := addr2line.ResolveString("0")
	orPanic(err)
	if r[0].Function != "??" || r[0].File != "??" || r[0].Line != 0 {
		t.Errorf("There should be no function at addr 0, has %+#v", r)
	}
}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func run(cmds ...string) {
	cmd := cmds[0]
	rest := cmds[1:]
	c := exec.Command(cmd, rest...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	orPanic(c.Run())
}

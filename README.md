# Go addr2line

Do you have memory addresses from an ELF file (say, kernel core dump)?

Do you want to find out where are they located in source programatically?

Including inline function information?

This library would give you this information, by running `addr2line` once,
and feeding it with the address to the standard input. Hence minimizing the
overhead for each address resolution.

In Go (golang)?

    package main

    import (
        "fmt"
        "log"
    	"github.com/elazarl/addr2line"
    )

    func main() {
    	a, err := addr2line.New("a.out")
        if err != nil {
            log.Fatalln("New", err)
        }
	rs, err := a.Resolve(0xff)
        if err != nil {
            log.Fatalln("Resolve", err)
        }
        fmt.Println(rs[0].Function, "@", rs[0].File, rs[0].Line)
        for _, r := range rs[1:] {
            fmt.Println("Inlined by", r.Function, "@", r.File, r.Line)
        }
    }

If you know that the source file's root directory was `/home/elazar/project`,
let `addr2line` know about it, it'll strip the prefix from the files.

    a, _ := addr2line.New("a.out")
    a.FilePrefix = []byte("/home/foo")


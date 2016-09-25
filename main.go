package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/mattn/go-encoding"
	"github.com/mattn/go-isatty"
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32")
	getConsoleCP = kernel32.NewProc("GetConsoleCP")
)

func terminate(pid int) error {
	dll, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer dll.Release()

	f, err := dll.FindProc("SetConsoleCtrlHandler")
	if err != nil {
		return err
	}
	r1, _, err := f.Call(0, 1)
	if r1 == 0 {
		return err
	}
	f, err = dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return err
	}
	r1, _, err = f.Call(syscall.CTRL_C_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	r1, _, err = f.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if r1 == 0 {
		return err
	}
	return nil
}

func run() int {
	i, _, _ := getConsoleCP.Call()
	cp := fmt.Sprintf("cp%d", i)

	var ie, oe string
	flag.StringVar(&ie, "i", cp, "encoding")
	flag.StringVar(&oe, "o", cp, "encoding")
	flag.Parse()
	ioenc := encoding.GetEncoding(oe)
	if ioenc == nil {
		fmt.Fprintln(os.Stderr, "unknown encoding")
		return 1
	}
	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	if isatty.IsTerminal(os.Stdout.Fd()) {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = ioenc.NewEncoder().Writer(os.Stdout)
		cmd.Stderr = ioenc.NewEncoder().Writer(os.Stderr)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_UNICODE_ENVIRONMENT | 0x00000200,
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	go io.Copy(in, os.Stdout)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		in.Close()
		terminate(cmd.Process.Pid)
	}()

	if err := cmd.Run(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			if status, ok := err.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			} else {
				panic(errors.New("Unimplemented for system where exec.ExitError.Sys() is not syscall.WaitStatus."))
			}
		}
	}
	return 0
}

func main() {
	os.Exit(run())
}

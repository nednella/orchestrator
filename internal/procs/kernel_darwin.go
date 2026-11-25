package procs

import "golang.org/x/sys/unix"

// zombie is SZOMB in <sys/proc.h>: a process that has exited and waits for its parent to read its status.
const zombie = 5

// kernel reads the processes from the kernel, as ps does: kern.proc.all for the list, kern.procargs2 for each one's
// arguments and environment.
type kernel struct{}

func (kernel) processes() ([]process, error) {
	list, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	procs := make([]process, 0, len(list))
	for _, k := range list {
		if k.Proc.P_stat == zombie {
			continue
		}
		start := k.Proc.P_starttime.Sec*1e6 + int64(k.Proc.P_starttime.Usec)
		procs = append(procs, process{pid: int(k.Proc.P_pid), ppid: int(k.Eproc.Ppid), pgid: int(k.Eproc.Pgid), start: start})
	}
	return procs, nil
}

func (kernel) args(pid int) (argv, env []string, err error) {
	buf, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, nil, err
	}
	argv, env = parseProcArgs(buf)
	return argv, env, nil
}

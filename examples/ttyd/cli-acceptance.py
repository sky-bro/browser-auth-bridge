#!/usr/bin/env python3
"""Opt-in live acceptance after installing the shared extension and logging in.

Uses the installed CLI origin configuration. Runs only prints, environment
changes in isolated shells and interruptible sleeps; never reads credentials.
Requires macOS/Linux PTY support and an authenticated ttyd session with Bash installed.
"""
import concurrent.futures,json,subprocess,os,pty,fcntl,termios,struct,signal,select,time
from pathlib import Path
binary=str(Path(__file__).resolve().parents[2]/'bin/bridge-ttyd')
def command(*args):
    return subprocess.run([binary,'exec','--',*args],capture_output=True,text=True,timeout=25)
r=command('bash','-c','printf "exit-check\\n"; exit 7');assert r.returncode==7 and r.stdout.strip()=='exit-check',(r.returncode,r.stdout,r.stderr)
literal="literal ' ; $(printf should-not-run) \" \\ end"
r=command('printf','%s\n',literal);assert r.returncode==0 and r.stdout.strip()==literal,(r.returncode,r.stdout,r.stderr)
def session(label):
    r=command('bash','-c','printf "%s:%s:%s\\n" '+label+' "$$" "${BRIDGE_LAB_STATE-unset}"; export BRIDGE_LAB_STATE='+label+'; sleep 1')
    assert r.returncode==0,(r.stdout,r.stderr)
    label,pid,state=r.stdout.strip().split(':');assert state=='unset';return pid
with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
    pids=list(pool.map(session,['one','two']))
assert pids[0]!=pids[1],pids
master,slave=pty.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',24,80,0,0))
p=subprocess.Popen([binary,'shell'],stdin=slave,stdout=slave,stderr=slave,start_new_session=True)
buffer=bytearray()
def until(marker,timeout=20):
    end=time.monotonic()+timeout
    while marker not in buffer:
        remaining=end-time.monotonic()
        assert remaining>0,('timeout',bytes(buffer[-1500:]))
        ready,_,_=select.select([master],[],[],remaining)
        if ready:
            chunk=os.read(master,65536);assert chunk,'terminal ended';buffer.extend(chunk)
try:
    os.write(master,b"printf '\\nINTERACTIVE_%s\\n' READY\n");until(b'INTERACTIVE_READY')
    fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',37,101,0,0));os.kill(p.pid,signal.SIGWINCH)
    # SIGWINCH delivery and stdin forwarding are independent. Verify the
    # eventual remote dimensions instead of racing the signal with one query.
    for attempt in range(10):
        offset=len(buffer)
        marker=('RESIZE_DONE_'+str(attempt)).encode()
        os.write(master,b"stty size; printf 'RESIZE_DONE_%s\\n' "+str(attempt).encode()+b"\n")
        until(marker,5)
        if b'37 101' in buffer[offset:]:
            break
    else:
        raise AssertionError('remote dimensions did not converge to 101x37')
    os.write(master,b"printf 'SLEEP_%s\\n' START; sleep 30\n");until(b'SLEEP_START')
    os.write(master,b'\x03');os.write(master,b"printf 'CTRL_C_%s\\n' OK\n");until(b'CTRL_C_OK',10)
    os.write(master,b'exit\n');p.wait(timeout=10);assert p.returncode==0,p.returncode
finally:
    if p.poll() is None:p.terminate();p.wait(timeout=10)
    os.close(master);os.close(slave)
print(json.dumps({'exit_code_7':True,'literal_argument_quoting':True,'concurrent_independent_sessions':2,'interactive_shell':True,'resize':'101x37','ctrl_c':True,'clean_exit':True}))

#!/usr/bin/env python3
"""Opt-in streaming acceptance against an authenticated WebShell Lab.

Runs only prints, sleeps and generated output; never reads credentials or files.
Requires the shared extension, a logged-in ttyd site, Bash and coreutils.
"""
import json
import os
from pathlib import Path
import select
import signal
import subprocess
import time

binary = str(Path(__file__).resolve().parents[2] / 'bin/bridge-ttyd')


def start(script, timeout='0'):
    return subprocess.Popen(
        [binary, 'exec', '--timeout', timeout, '--', 'bash', '-c', script],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=0)


def read_ready(p, expected=b'ready\r\n'):
    output = bytearray()
    deadline = time.monotonic() + 30
    while not output.endswith(expected):
        remaining = deadline - time.monotonic()
        assert remaining > 0, ('first output timed out', bytes(output))
        ready, _, _ = select.select([p.stdout], [], [], remaining)
        assert ready, 'first output timed out'
        chunk = os.read(p.stdout.fileno(), 4096)
        assert chunk, ('early EOF', p.wait(), p.stderr.read())
        output.extend(chunk)
    return bytes(output)


def cleanup(p):
    if p.poll() is None:
        p.kill()
        p.wait(timeout=5)
    p.stdout.close()
    p.stderr.close()


p = start('printf "ready\\n"; sleep 2; printf "late\\n"; exit 7')
try:
    read_ready(p)
    assert p.poll() is None, 'first output was delayed until exit'
    rest, errors = p.communicate(timeout=10)
    assert p.returncode == 7 and rest == b'late\r\n', (p.returncode, rest, errors)
finally:
    cleanup(p)

p = start("head -c 20971520 /dev/zero | tr '\\000' x", '45s')
try:
    count = 0
    while chunk := p.stdout.read(32768):
        assert chunk == b'x' * len(chunk), 'protocol marker or unexpected output'
        count += len(chunk)
    assert p.wait(timeout=10) == 0 and count == 20 << 20, (p.returncode, count, p.stderr.read())
finally:
    cleanup(p)

p = start('printf "ready\\n"; sleep 30', '5s')
try:
    read_ready(p)
    p.communicate(timeout=10)
    assert p.returncode == 124, p.returncode
finally:
    cleanup(p)

# Stop consuming after the first line, so cancellation also exercises a blocked
# local stdout pipe instead of testing only an idle remote read.
p = start('printf "ready\\n"; sleep 1; yes x')
try:
    read_ready(p)
    time.sleep(2)
    p.send_signal(signal.SIGINT)
    assert p.wait(timeout=5) == 130, p.returncode
finally:
    cleanup(p)

print(json.dumps({'output_before_exit': True, 'exit_code': 7,
                  'streamed_bytes': count, 'timeout_exit': 124,
                  'cancel_with_stalled_consumer': 130}))

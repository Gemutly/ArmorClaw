# Office HMAC Crash Diagnosis

## Confirmed Hypothesis

Choose exactly one primary hypothesis:

- H1: Secret file does not exist
- H2: Parent directory permissions block traversal
- H3: AppArmor/SELinux blocks access
- H4: Race condition / secret created after sidecar starts
- H5: Docker bind-mount/inode issue
- H6: Other — describe

**Confirmed hypothesis:** H2

The container runs as root (uid 0) with ALL capabilities dropped. The bind-mounted volume
directory `/run/armorclaw` is mode 770 owned by uid 10001:10001. Without `CAP_DAC_OVERRIDE`,
even uid 0 cannot traverse a 770 directory owned by another uid. This causes `PermissionError`
when the Python code attempts to `open()` the HMAC secret file.

## Evidence Summary

### Crash Logs

Command:
```bash
docker logs armorclaw-sidecar-office --tail=100
```

Observed result:
```text
Traceback (most recent call last):
  File "/app/worker.py", line 199, in <module>
    serve()
  File "/app/worker.py", line 167, in serve
    shared_secret = load_shared_secret()
                    ^^^^^^^^^^^^^^^^^^^^
  File "/app/interceptor.py", line 34, in load_shared_secret
    with open(secret_path, "r") as f:
         ^^^^^^^^^^^^^^^^^^^^^^
PermissionError: [Errno 13] Permission denied: '/run/armorclaw/secrets/office-hmac'
```

Repeated in crash loop. Exit code 1, restart policy: unless-stopped.

### Container Configuration Mismatch

Running container (actual):
```text
Image:        mikegemut/sidecar-office:beato
User:         (empty → root uid 0)
CapDrop:      ALL
SecurityOpt:  [no-new-privileges:true]
AppArmor:     (none — compose profile armorclaw-office-worker does not exist)
Env:          SECRET_PATH=/run/armorclaw/secrets/office-hmac
Mounts:       /var/lib/docker/volumes/d2772372.../_data → /run/armorclaw (RW, rslave)
ReadonlyRootfs: false
```

Intended compose file (`deploy/docker-compose.sidecar-py.yml`):
```text
Image:        armorclaw/sidecar-office:latest
User:         10001:10001
CapDrop:      ALL
SecurityOpt:  [no-new-privileges:true, apparmor=armorclaw-office-worker]
Env:          SIDECAR_SOCKET, SIDECAR_SECRET_FILE=/run/secrets/shared_secret
Mounts:       /run/armorclaw:/run/armorclaw
              /run/armorclaw/secrets/office-hmac:/run/secrets/shared_secret:ro
ReadonlyRootfs: true
```

**Critical differences:**
1. `user: "10001:10001"` is MISSING from the running container
2. HMAC secret file bind-mount (`/run/secrets/shared_secret:ro`) is MISSING
3. AppArmor profile is MISSING (compose specifies it, but profile doesn't exist on host)
4. Different image tag (`beato` vs `latest`)
5. Different env var name (`SECRET_PATH` vs `SIDECAR_SECRET_FILE`)
6. ReadonlyRootfs is false in running container

### Host Path

Command:
```bash
namei -l /run/armorclaw/secrets/office-hmac
stat -c "%a %U %G %u %g %n" /run/armorclaw /run/armorclaw/secrets /run/armorclaw/secrets/office-hmac
```

Observed result:
```text
namei -l /run/armorclaw/secrets/office-hmac
f: /run/armorclaw/secrets/office-hmac
drwxr-xr-x root  root  /
drwxr-xr-x root  root  run
drwxrwx--- 10001 10001 armorclaw        ← 770, uid 10001 — NO world-readable bit
drwxr-xr-x root  root  secrets
-r-------- 10001 10001 office-hmac

stat output:
770 UNKNOWN UNKNOWN 10001 10001 /run/armorclaw
755 root root 0 0 /run/armorclaw/secrets
400 UNKNOWN UNKNOWN 10001 10001 /run/armorclaw/secrets/office-hmac
```

Filesystem: `/run/armorclaw` is on tmpfs (part of `/run`).

Docker volume data (what the container actually sees):
```text
770 UNKNOWN UNKNOWN 10001 10001 /var/lib/docker/volumes/d2772372.../_data
755 root root 0 0 /var/lib/docker/volumes/d2772372.../_data/secrets
644 UNKNOWN UNKNOWN 10001 10001 /var/lib/docker/volumes/d2772372.../_data/secrets/office-hmac
```

Note: Volume file is mode 644 (readable by all), but the parent directory is 770/10001:10001,
which blocks traversal for any uid != 10001 without CAP_DAC_OVERRIDE.

### Container Path — Reproduction Test

Command (root + CapDrop ALL — reproduces crash):
```bash
docker run --rm --cap-drop ALL --security-opt no-new-privileges:true \
  -v /var/lib/docker/volumes/d2772372.../_data:/run/armorclaw \
  --entrypoint sh mikegemut/sidecar-office:beato -c "
  id
  ls -ld /run/armorclaw 2>&1
  cat /run/armorclaw/secrets/office-hmac 2>&1
  stat /run/armorclaw/secrets/office-hmac 2>&1
"
```

Observed result:
```text
uid=0(root) gid=0(root) groups=0(root)
drwxrwx--- 4 10001 10001 4096 May 16 13:29 /run/armorclaw
cat: /run/armorclaw/secrets/office-hmac: Permission denied
stat: cannot statx '/run/armorclaw/secrets/office-hmac': Permission denied
```

Command (uid 10001 — confirms fix):
```bash
docker run --rm --user "10001:10001" --cap-drop ALL \
  --security-opt no-new-privileges:true \
  -v /var/lib/docker/volumes/d2772372.../_data:/run/armorclaw \
  --entrypoint sh mikegemut/sidecar-office:beato -c "
  id
  cat /run/armorclaw/secrets/office-hmac
"
```

Observed result:
```text
uid=10001 gid=10001 groups=10001
60e264db...
```

### AppArmor / Kernel Denials

Command:
```bash
dmesg | grep -i -E "apparmor|denied|audit"
journalctl -k | grep -i -E "apparmor|denied|audit"
```

Observed result:
```text
[1374011.319072] audit: type=1400 audit(1778942379.464:125): apparmor="DENIED"
  operation="change_onexec" class="file" info="label not found" error=-2
  profile="unconfined" name="armorclaw-office-worker" pid=2088539
  comm="runc:[2:INIT]"
```

AppArmor status:
```text
   /opt/armorclaw/armorclaw-bridge (2062484) docker-default
```

The `armorclaw-office-worker` AppArmor profile does NOT exist on the host. Docker cannot
apply it, causing the compose `run` command to fail entirely. The running container was
started WITHOUT AppArmor confinement.

## Why This Hypothesis Is Confirmed

- **The crash is PermissionError, not FileNotFoundError.** The file exists at the expected path inside the container volume. The container cannot READ it due to directory traversal denial.
- **Directory `/run/armorclaw` is mode 770/10001:10001.** The "other" permission bits are 0. Only uid 10001 or members of gid 10001 can traverse it.
- **Container runs as root (uid 0) with ALL capabilities dropped.** Without `CAP_DAC_OVERRIDE`, even uid 0 must respect DAC permission bits. The kernel denies traversal because "other" bits are 0.
- **Reproduction test confirms the exact error.** Running as root + CapDrop ALL produces the identical `Permission denied`. Running as `--user "10001:10001"` + CapDrop ALL succeeds.
- **The compose file specifies `user: "10001:10001"` which would fix this.** The running container was started without this directive, likely from an older deployment script or manual `docker run`.
- **H1 (file missing) is ruled out:** the file exists in the Docker volume (mode 644 at volume path).
- **H3 (AppArmor) is ruled out:** the running container has NO AppArmor confinement; the PermissionError is a standard DAC denial.
- **H4 (race condition) is ruled out:** the file exists with stable permissions; the crash is deterministic on every start.
- **H5 (bind-mount issue) is ruled out:** the bind mount works correctly; the problem is purely permission-based.

## Implication for T4

T4 must ensure the sidecar-office container is started with `user: "10001:10001"` (or equivalent UID mapping) so the process can traverse the 770 directory and read the HMAC secret. Specifically:

1. **Deploy the container using the compose file as-is** — it already specifies `user: "10001:10001"`.
2. **Create the missing AppArmor profile `armorclaw-office-worker`** on the host, OR remove the `apparmor=armorclaw-office-worker` security_opt from the compose file so the container can start at all.
3. **The HMAC secret file must exist BEFORE the container starts.** Bridge code must call `GenerateSharedSecret()` to create `/run/armorclaw/secrets/office-hmac` with proper ownership (10001:10001) and mode (400).
4. **The compose file env var is `SIDECAR_SECRET_FILE=/run/secrets/shared_secret`** but interceptor.py reads `SECRET_PATH`. These must be aligned, or the code must fall back correctly.

## Do Not Do

- Do NOT chmod /run/armorclaw to 755 — this would weaken security by making the secrets directory world-traversable.
- Do NOT remove CapDrop ALL — this is a security hardening measure.
- Do NOT add CAP_DAC_OVERRIDE — this undermines the capability dropping.
- Do NOT change the file to mode 644 or world-readable — the 400 mode is correct for a secret.
- Do NOT run the container as root without capability drops — the compose file's `user: "10001:10001"` is the correct approach.
- Do NOT ignore the AppArmor profile missing issue — the compose file will fail to start containers until the profile is created or the security_opt is removed.

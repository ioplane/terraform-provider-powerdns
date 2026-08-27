# Development

Everything runs inside the dev container. The host needs Podman,
`podman-compose`, Task, Python 3.12 and GNU coreutils `sha256sum` — nothing
else.

Python is on that list because the host-side automation is Python: the lab
lifecycle, the end-to-end fixture, the worktree helper and every check the gate
runs. It is not an addition in practice — `podman-compose` is itself a Python
program and cannot be installed without an interpreter — but the list said
"nothing else", and a prerequisite that is only true by implication is one a
contributor discovers by failing.

`sha256sum` is the checked host-side prerequisite used to derive a
collision-resistant linked-worktree identity before Compose runs. The helper
rejects a failed command or anything other than one exact 64-character
lowercase hexadecimal digest; it never emits a partial suffix.

## First run

```sh
task up        # build and start the dev container
task recreate  # explicitly replace it after a pinned Go version change
task shell     # a shell inside it
task versions  # confirm the pinned toolchain
```

The Go 1.27.0 image is fully qualified and **pinned by digest**, with every tool
pinned by build argument. Versions live in
`deployments/containers/Containerfile.dev`; workflow pins are checked against
it, while Compose builds the resulting worktree-specific local image tag. The
active identity and cache contract is
[ADR 0010](adr/0010-go-1.27-development-toolchain.md).

`task up` is non-destructive. If a rebuilt local image tag changes the pinned Go
runtime, container-backed tasks fail closed and direct the user to the explicit
`task recreate` lifecycle action. It completes the image build before replacing
the exact checkout container, and refuses removal unless its Compose project
label and canonical `/app` bind both belong to the current worktree. Compose
projects and local image tags are named per worktree so replacement cannot
select another checkout's resources or change its image tag.

For a linked worktree, Task derives the suffix from the sanitized lowercase
worktree-root basename (at most 48 characters) and a 12-hex SHA-256 prefix of
the canonical full root path. The suffix is therefore unchanged when Task is
called from a subdirectory and distinct for equal base names at different
paths. The primary checkout keeps the empty suffix and its historical object
names. Run `task --dry --verbose up` for a read-only check showing the resolved
current-checkout project and container identity before authorizing recreation.

## The lab

```sh
task lab:up       # five services, wait for every API
task lab:verify   # assert versions match the pinned references
task lab:status   # container state and reported versions
task lab:down     # remove, including volumes
```

| Service | Endpoint | Why it exists |
| --- | --- | --- |
| `pdns-lab-auth-pg` | `:18081` | Authoritative on PostgreSQL — the common deployment |
| `pdns-lab-auth-lmdb` | `:18091` | Authoritative on LMDB — the **only** backend implementing views and networks |
| `pdns-lab-rec` | `:18082` | Recursor with `api_dir` — without it every write returns 422 |
| `pdns-lab-dnsdist` | `:18083` | dnsdist — the only place its two write operations exist |
| `pdns-lab-pg` | `:15432` | disposable PostgreSQL 18.6 backend for the first |

Five is the minimum that covers the provider, not thoroughness. `lab:verify`
asserts the fixture is the one the tests were written against, so a silently
upgraded image is caught before it produces a confusing failure.

PostgreSQL 18 keeps `PGDATA` below its declared `/var/lib/postgresql` volume.
Compose overlays that destination with a 512 MiB tmpfs using
`nosuid,nodev,noexec`, so `task lab:up` always initializes a clean cluster from
`test/lab/schema.pgsql.sql` and cannot leave an anonymous database volume. This
proves fresh initialization, not `pg_upgrade` or durable-data compatibility.

## The daily loop

```sh
task build
task test           # unit, race detector on
task test:contract  # recorded fixtures, no containers
task lint
task all            # the pre-merge gate
task verify         # all, plus lab acceptance on both backends
```

## Troubleshooting

**A task fails with "dev container is not running".** That is the guard doing
its job: `task up`.

**Views or networks return 422 on `auth-pg`.** Expected — they require LMDB.
Point `PDNS_SERVER_URL` at `:18091`.

**Recursor writes return 422.** `webservice.api_dir` is unset. The lab sets it;
a hand-rolled recursor will not.

**dnsdist `PUT` returns 405.** `setAPIWritable` is not enabled. Setting
`apiConfigDir` alone is not enough — `isMethodAllowed()` checks `d_apiReadWrite`
before it looks at the path.

**dnsdist cache flush returns 404.** The pool has no packet cache. That is
about the pool, not the endpoint.

**Rootless Podman cannot bind port 53.** The lab listens high and publishes
high. A property of the fixture.

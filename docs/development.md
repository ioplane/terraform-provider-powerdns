# Development

Everything runs inside the dev container. The host needs Podman,
`podman-compose` and Task — nothing else.

## First run

```sh
task up        # build and start the dev container
task shell     # a shell inside it
task versions  # confirm the pinned toolchain
```

The image is `golang:1.26-trixie` **pinned by digest**, with every tool pinned
by build argument. Versions live in one place,
`deployments/containers/Containerfile.dev`, mirrored into
`deployments/compose/compose.dev.yml`.

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
| `pdns-lab-pg` | `:15432` | backend for the first |

Five is the minimum that covers the provider, not thoroughness. `lab:verify`
asserts the fixture is the one the tests were written against, so a silently
upgraded image is caught before it produces a confusing failure.

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

# plugin-user

Unix account provisioning for OpenCharly — the `user:` verb.

The verb is a multi-role state-provision verb: it **checks** an account with
`getent passwd` through the live check engine and compares uid/gid/home/shell,
and it **acts** by rendering an idempotent `useradd`. There are no matchers — it
does a direct field comparison.

It is a host-coupled verb on the SDK kit contract (`CheckVerbProvider` +
`ProvisionActor`), so it is **compiled-in only**.

## What it provides

| Capability | Surface |
|---|---|
| `verb:user` | the `user:` typed step — probe an account and render its creation |

## How to use it

Compose the plugin candy in a box or check bed's `candy:` list:

```yaml
- '@github.com/opencharly/plugin-user/candy/plugin-user:<tag>'
```

Then author the verb in a plan:

```yaml
- check: the root account exists with uid 0
  id: user-root
  user: {user: root, uid: 0}
  context: [runtime]
```

## Layout

- `candy/plugin-user/` — the plugin module: `plugin.go` (the verb +
  `NewCheckVerb()`/`NewMeta()`), `groups_test.go` (group handling), `schema/user.cue`
  (the self-contained `#UserInput`), `params/cue_types_gen.go`, `cmd/serve/main.go`.
- `charly.yml` — the root project manifest (`discover: candy`).
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-internals:plugin` — the plugin/provider model and the
  multi-role state-provision contract this verb follows. This candy carries no
  `skill:` entity of its own; the gap is tracked in
  [opencharly/opencharly#291](https://github.com/opencharly/opencharly/issues/291).
- `/charly-core:service` — the sibling init-agnostic service-provision verb.
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.

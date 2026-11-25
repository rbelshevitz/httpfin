# Baseline updater safety

The browser updater must be treated as a data pipeline, not as a firewall
control plane.

```text
browser capture → candidate baseline → review → active baseline
                                      ↘ rollback
```

The updater must never write directly to the active file, emit a Fail2ban
`decision=deny`, or receive production credentials and cookies. Unknown,
missing, stale, or changed observations must produce `unknown`/`observe`.

Required controls:

- pinned browser and runner versions;
- isolated network and disposable profile data;
- schema validation and bounded fields;
- repeated captures before accepting a change;
- human-reviewed diff;
- previous active baseline retained for rollback;
- activation audit record;
- explicit source and tool version for every entry.

The current baseline loader only reads `httpfin_baseline_file`; it does not
modify the file or activate a new baseline. This is intentional.

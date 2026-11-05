# Limitations

- This is a local contract snapshot utility, not server integration or a
  production account/session system.
- Schema numbers and categories are proposal shape constraints only. They do
  not define enabled slots, login policy, progression behavior, or operational
  session limits.
- No sessions are created or activated, and there is no concurrent
  session/mutation enforcement.
- Inventory/storage, transactional gameplay behavior, PvP, matchmaking, and
  category competition rules are outside scope.
- Character-reference uniqueness is enforced only within each account snapshot.
  The contract does not state that references are globally unique.
- Snapshots are supplied by the caller; this module does not connect to a
  database, game server, or live WYD environment.

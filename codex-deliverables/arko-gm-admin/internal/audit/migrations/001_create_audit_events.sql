CREATE TABLE arko_admin_audit_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor TEXT NOT NULL,
    actor_role TEXT NOT NULL,
    action TEXT NOT NULL,
    capability TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL CHECK (outcome IN ('requested', 'approved', 'denied', 'failed')),
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX arko_admin_audit_events_occurred_at_id_idx
    ON arko_admin_audit_events (occurred_at DESC, id DESC);
CREATE INDEX arko_admin_audit_events_actor_occurred_at_idx
    ON arko_admin_audit_events (actor, occurred_at DESC, id DESC);
CREATE INDEX arko_admin_audit_events_action_occurred_at_idx
    ON arko_admin_audit_events (action, occurred_at DESC, id DESC);
CREATE INDEX arko_admin_audit_events_target_occurred_at_idx
    ON arko_admin_audit_events (target_type, target_id, occurred_at DESC, id DESC);

CREATE FUNCTION arko_admin_reject_audit_event_mutation() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'arko_admin_audit_events is append-only'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER audit_events_reject_update_delete
    BEFORE UPDATE OR DELETE ON arko_admin_audit_events
    FOR EACH ROW EXECUTE FUNCTION arko_admin_reject_audit_event_mutation();

CREATE TRIGGER audit_events_reject_truncate
    BEFORE TRUNCATE ON arko_admin_audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION arko_admin_reject_audit_event_mutation();

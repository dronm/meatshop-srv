-- Simple function to get column alias
CREATE OR REPLACE FUNCTION audit_log_col_alias(
    p_table_name TEXT,
    p_column_name TEXT
) RETURNS TEXT AS $$
DECLARE
    v_alias TEXT;
BEGIN
    SELECT column_alias INTO v_alias
    FROM audit_column_aliases
    WHERE table_name = p_table_name
      AND column_name = p_column_name
      AND is_active = TRUE;
    
    RETURN COALESCE(v_alias, p_column_name); -- Fallback to column name
END;
$$ LANGUAGE plpgsql STABLE;



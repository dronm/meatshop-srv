--DROP FUNCTION audit_log_process();

-- Modified trigger for compact storage
CREATE OR REPLACE FUNCTION audit_log_process()
RETURNS TRIGGER AS $$
DECLARE
    changes JSONB := '[]'::JSONB;
    key TEXT;
    old_val JSONB;
    new_val JSONB;
    record_id TEXT;
    app_user_name TEXT;
    app_user_role TEXT;
    column_alias TEXT;
    fk_table_name TEXT;
    fk_ref_func_name TEXT;
    fk_ref_func_exists BOOLEAN;
    old_descr TEXT;
    new_descr TEXT;
    ref_result JSON;
    pk_columns TEXT[];
    pk_value TEXT := '';
    col_name TEXT;
    temp_val TEXT;
BEGIN
    -- Safely get session variables
    BEGIN
        app_user_name := current_setting('app.user_name', true);
        app_user_role := current_setting('app.user_role', true);
    EXCEPTION WHEN undefined_object THEN
        app_user_name := NULL;
        app_user_role := NULL;
    END;
    
    -- Process role and user name formatting
    IF app_user_role IS NOT NULL AND app_user_role <> '' THEN
        app_user_role := enum_role_types_val(app_user_role::role_types, 'ru');
        app_user_name := COALESCE(app_user_name, '') || ' (' || app_user_role || ')';
    END IF;
    
    -- Fallback if no app user name
    app_user_name := COALESCE(app_user_name, current_user);

    -- Get primary key columns dynamically
    SELECT array_agg(a.attname ORDER BY k.ordinal_position)
    INTO pk_columns
    FROM pg_index i
    JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
    JOIN pg_class c ON c.oid = i.indrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN information_schema.key_column_usage k 
        ON k.table_name = c.relname 
        AND k.table_schema = n.nspname 
        AND k.column_name = a.attname
    WHERE i.indisprimary
        AND c.relname = TG_TABLE_NAME
        AND n.nspname = TG_TABLE_SCHEMA
    GROUP BY i.indrelid;

    -- Build record identifier using primary key columns
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        IF pk_columns IS NOT NULL AND array_length(pk_columns, 1) > 0 THEN
            -- Construct composite key value
            FOREACH col_name IN ARRAY pk_columns
            LOOP
                EXECUTE format('SELECT ($1).%I::TEXT', col_name)
                INTO temp_val
                USING OLD;
                
                IF pk_value = '' THEN
                    pk_value := temp_val;
                ELSE
                    pk_value := pk_value || '|' || temp_val;
                END IF;
            END LOOP;
            record_id := pk_value;
        ELSE
            -- Fallback: try to get an ID field or use the whole record
            record_id := COALESCE(
                (OLD).id::TEXT, 
                row_to_json(OLD)->>'id', 
                row_to_json(OLD)::TEXT
            );
        END IF;
    ELSE -- INSERT
        IF pk_columns IS NOT NULL AND array_length(pk_columns, 1) > 0 THEN
            -- Construct composite key value
            FOREACH col_name IN ARRAY pk_columns
            LOOP
                EXECUTE format('SELECT ($1).%I::TEXT', col_name)
                INTO temp_val
                USING NEW;
                
                IF pk_value = '' THEN
                    pk_value := temp_val;
                ELSE
                    pk_value := pk_value || '|' || temp_val;
                END IF;
            END LOOP;
            record_id := pk_value;
        ELSE
            -- Fallback: try to get an ID field or use the whole record
            record_id := COALESCE(
                (NEW).id::TEXT, 
                row_to_json(NEW)->>'id', 
                row_to_json(NEW)::TEXT
            );
        END IF;
    END IF;
    
    -- Build changes array
    IF TG_OP = 'UPDATE' THEN
        -- Check all columns from both old and new
        FOR key IN (
            SELECT DISTINCT unnest(ARRAY(
                SELECT jsonb_object_keys(to_jsonb(OLD))
                UNION
                SELECT jsonb_object_keys(to_jsonb(NEW))
            ))
        )
        LOOP
            old_val := to_jsonb(OLD) -> key;
            new_val := to_jsonb(NEW) -> key;
            
            -- Check if value changed
            IF old_val IS DISTINCT FROM new_val THEN
                -- Get column alias
                column_alias := audit_log_col_alias(TG_TABLE_NAME, key);
                
                -- Initialize description values
                old_descr := NULL;
                new_descr := NULL;
                
                -- Check if this column is a foreign key and find the referenced table
                SELECT ccu.table_name 
                INTO fk_table_name
                FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu 
                    ON tc.constraint_name = kcu.constraint_name
                    AND tc.table_schema = kcu.table_schema
                JOIN information_schema.constraint_column_usage ccu 
                    ON ccu.constraint_name = tc.constraint_name
                    AND ccu.table_schema = tc.table_schema
                WHERE tc.constraint_type = 'FOREIGN KEY'
                    AND tc.table_schema = TG_TABLE_SCHEMA
                    AND tc.table_name = TG_TABLE_NAME
                    AND kcu.column_name = key
                LIMIT 1;
                
                -- If it's a foreign key, try to get descriptions
                IF fk_table_name IS NOT NULL THEN
                    -- Try to find the reference function (pattern: referenced_table + '_ref')
                    fk_ref_func_name := fk_table_name || '_ref';
                    
                    -- Check if function exists - now looking for JSON return type
                    SELECT EXISTS (
                        SELECT 1 
                        FROM pg_proc p
                        JOIN pg_namespace n ON p.pronamespace = n.oid
                        WHERE n.nspname = current_schema()
                        AND p.proname = fk_ref_func_name
                        AND pg_get_function_result(p.oid) IN ('json', 'jsonb')
                    ) INTO fk_ref_func_exists;
                    
                    -- Get description for old value if exists
                    IF fk_ref_func_exists AND old_val IS NOT NULL AND old_val::TEXT <> 'null' THEN
                        BEGIN
                            EXECUTE format('SELECT %I((SELECT %I FROM %I WHERE id = $1))', 
                                          fk_ref_func_name, fk_table_name, fk_table_name)
                            INTO ref_result
                            USING old_val::text::integer;
                            
                            IF ref_result IS NOT NULL THEN
                                old_descr := ref_result->>'descr';
                            END IF;
                        EXCEPTION WHEN OTHERS THEN
                            -- If function call fails, just use the ID
                            old_descr := old_val::text;
                        END;
                    END IF;
                    
                    -- Get description for new value if exists
                    IF fk_ref_func_exists AND new_val IS NOT NULL AND new_val::TEXT <> 'null' THEN
                        BEGIN
                            EXECUTE format('SELECT %I((SELECT %I FROM %I WHERE id = $1))', 
                                          fk_ref_func_name, fk_table_name, fk_table_name)
                            INTO ref_result
                            USING new_val::text::integer;
                            
                            IF ref_result IS NOT NULL THEN
                                new_descr := ref_result->>'descr';
                            END IF;
                        EXCEPTION WHEN OTHERS THEN
                            -- If function call fails, just use the ID
                            new_descr := new_val::text;
                        END;
                    END IF;
                END IF;
                
                -- Add to changes array with descriptions if available
                changes := changes || jsonb_build_object(
                    'col', key,              -- Original column name
                    'alias', column_alias,   -- Display alias
                    'old', old_val,
                    'new', new_val,
                    'old_descr', old_descr,  -- Foreign key description for old value
                    'new_descr', new_descr   -- Foreign key description for new value
                );
            END IF;
        END LOOP;
        
        -- Only insert if changes exist
        IF jsonb_array_length(changes) > 0 THEN
            INSERT INTO audit_log(
                table_name, 
                record_id, 
                operation, 
                changes, 
                changed_by,
                changed_at
            ) VALUES (
                TG_TABLE_NAME, 
                record_id, 
                'U', 
                changes, 
                app_user_name,
                NOW()
            );
        END IF;
        
    ELSIF TG_OP = 'INSERT' THEN
        -- For INSERT, include all columns
        FOR key IN (SELECT jsonb_object_keys(to_jsonb(NEW)))
        LOOP
            new_val := to_jsonb(NEW) -> key;
            
            -- Get column alias
            column_alias := audit_log_col_alias(TG_TABLE_NAME, key);
            
            -- Initialize description value
            new_descr := NULL;
            
            -- Check if this column is a foreign key and find the referenced table
            SELECT ccu.table_name 
            INTO fk_table_name
            FROM information_schema.table_constraints tc
            JOIN information_schema.key_column_usage kcu 
                ON tc.constraint_name = kcu.constraint_name
                AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage ccu 
                ON ccu.constraint_name = tc.constraint_name
                AND ccu.table_schema = tc.table_schema
            WHERE tc.constraint_type = 'FOREIGN KEY'
                AND tc.table_schema = TG_TABLE_SCHEMA
                AND tc.table_name = TG_TABLE_NAME
                AND kcu.column_name = key
            LIMIT 1;
            
            -- If it's a foreign key, try to get description
            IF fk_table_name IS NOT NULL AND new_val IS NOT NULL AND new_val::TEXT <> 'null' THEN
                -- Try to find the reference function (pattern: referenced_table + '_ref')
                fk_ref_func_name := fk_table_name || '_ref';
                
                -- Check if function exists - now looking for JSON return type
                SELECT EXISTS (
                    SELECT 1 
                    FROM pg_proc p
                    JOIN pg_namespace n ON p.pronamespace = n.oid
                    WHERE n.nspname = current_schema()
                    AND p.proname = fk_ref_func_name
                    AND pg_get_function_result(p.oid) IN ('json', 'jsonb')
                ) INTO fk_ref_func_exists;
                
                -- Get description if function exists
                IF fk_ref_func_exists THEN
                    BEGIN
                        EXECUTE format('SELECT %I((SELECT %I FROM %I WHERE id = $1))', 
                                      fk_ref_func_name, fk_table_name, fk_table_name)
                        INTO ref_result
                        USING new_val::text::integer;
                        
                        IF ref_result IS NOT NULL THEN
                            new_descr := ref_result->>'descr';
                        END IF;
                    EXCEPTION WHEN OTHERS THEN
                        -- If function call fails, just use the ID
                        new_descr := new_val::text;
                    END;
                END IF;
            END IF;
            
            changes := changes || jsonb_build_object(
                'col', key,
                'alias', column_alias,
                'old', NULL,
                'new', new_val,
                'new_descr', new_descr  -- Foreign key description for new value
            );
        END LOOP;
        
        INSERT INTO audit_log(
            table_name, 
            record_id, 
            operation, 
            changes, 
            changed_by,
            changed_at
        ) VALUES (
            TG_TABLE_NAME, 
            record_id, 
            'I', 
            changes, 
            app_user_name,
            NOW()
        );
        
    ELSIF TG_OP = 'DELETE' THEN
        -- For DELETE, include all columns
        FOR key IN (SELECT jsonb_object_keys(to_jsonb(OLD)))
        LOOP
            old_val := to_jsonb(OLD) -> key;
            
            -- Get column alias
            column_alias := audit_log_col_alias(TG_TABLE_NAME, key);
            
            -- Initialize description value
            old_descr := NULL;
            
            -- Check if this column is a foreign key and find the referenced table
            SELECT ccu.table_name 
            INTO fk_table_name
            FROM information_schema.table_constraints tc
            JOIN information_schema.key_column_usage kcu 
                ON tc.constraint_name = kcu.constraint_name
                AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage ccu 
                ON ccu.constraint_name = tc.constraint_name
                AND ccu.table_schema = tc.table_schema
            WHERE tc.constraint_type = 'FOREIGN KEY'
                AND tc.table_schema = TG_TABLE_SCHEMA
                AND tc.table_name = TG_TABLE_NAME
                AND kcu.column_name = key
            LIMIT 1;
            
            -- If it's a foreign key, try to get description
            IF fk_table_name IS NOT NULL AND old_val IS NOT NULL AND old_val::TEXT <> 'null' THEN
                -- Try to find the reference function (pattern: referenced_table + '_ref')
                fk_ref_func_name := fk_table_name || '_ref';
                
                -- Check if function exists - now looking for JSON return type
                SELECT EXISTS (
                    SELECT 1 
                    FROM pg_proc p
                    JOIN pg_namespace n ON p.pronamespace = n.oid
                    WHERE n.nspname = current_schema()
                    AND p.proname = fk_ref_func_name
                    AND pg_get_function_result(p.oid) IN ('json', 'jsonb')
                ) INTO fk_ref_func_exists;
                
                -- Get description if function exists
                IF fk_ref_func_exists THEN
                    BEGIN
                        EXECUTE format('SELECT %I((SELECT %I FROM %I WHERE id = $1))', 
                                      fk_ref_func_name, fk_table_name, fk_table_name)
                        INTO ref_result
                        USING old_val::text::integer;
                        
                        IF ref_result IS NOT NULL THEN
                            old_descr := ref_result->>'descr';
                        END IF;
                    EXCEPTION WHEN OTHERS THEN
                        -- If function call fails, just use the ID
                        old_descr := old_val::text;
                    END;
                END IF;
            END IF;
            
            changes := changes || jsonb_build_object(
                'col', key,
                'alias', column_alias,
                'old', old_val,
                'new', NULL,
                'old_descr', old_descr  -- Foreign key description for old value
            );
        END LOOP;
        
        INSERT INTO audit_log(
            table_name, 
            record_id, 
            operation, 
            changes, 
            changed_by,
            changed_at
        ) VALUES (
            TG_TABLE_NAME, 
            record_id, 
            'D', 
            changes, 
            app_user_name,
            NOW()
        );
    END IF;
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;



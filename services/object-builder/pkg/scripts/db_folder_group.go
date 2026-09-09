package scripts

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func DeleteFolderGroup(db *pgxpool.Pool) error {
	query := `BEGIN;
		DO
		$$
		DECLARE
	 		ids       uuid[];
	 		ids_text  text[];
	 		rec       RECORD;
	 		has_relation boolean;
		BEGIN
	
		SELECT COALESCE(array_agg(id)::uuid[], '{}'::uuid[]),
	        COALESCE(array_agg(id::text)::text[], '{}'::text[])
	 	INTO ids, ids_text
	 	FROM "field"
	 	WHERE slug = 'folder_id';
	
	 	FOR rec IN
			SELECT quote_ident(n.nspname) AS schemaname,
				quote_ident(c.relname) AS relname
	   	FROM pg_catalog.pg_attribute a
	   	JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
	   	JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	   	WHERE a.attname = 'folder_id'
	     	AND a.attnum > 0
	     	AND NOT a.attisdropped
	     	AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
	     	AND c.relkind IN ('r', 'p')                   -- только таблицы
	     	AND COALESCE(c.relispartition, false) = false -- дочерние партиции не трогаем
	 	LOOP
	   	BEGIN
	     	EXECUTE format('ALTER TABLE %s.%s DROP COLUMN IF EXISTS folder_id CASCADE;', rec.schemaname, rec.relname);
	     	RAISE NOTICE 'Dropped column folder_id in %.%', rec.schemaname, rec.relname;
	   		EXCEPTION WHEN OTHERS THEN
	     	RAISE WARNING 'Failed to drop folder_id in %.%: %', rec.schemaname, rec.relname, SQLERRM;
	   	END;
	 	END LOOP;
	
	 	SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
			AND table_name = 'relation'
		)
	 	INTO has_relation;
	
	 	IF has_relation AND array_length(ids, 1) IS NOT NULL THEN
	
		UPDATE "relation" r
	   	SET view_fields = (
			SELECT CASE WHEN COUNT(*) = 0 THEN NULL ELSE array_agg(e) END
	     	FROM unnest(r.view_fields) e
	     	WHERE NOT (e::text = ANY (ids_text))
	   	)
	   	WHERE EXISTS (
			SELECT 1
				FROM information_schema.columns
	           	WHERE table_name = 'relation'
	             	AND column_name = 'view_fields'
	             	AND udt_name = '_uuid'
	         )
		AND r.view_fields IS NOT NULL
		AND EXISTS (SELECT 1 FROM unnest(r.view_fields) e WHERE e::text = ANY (ids_text));
	
	   	FOR rec IN
	     	SELECT a.attname AS colname
	     	FROM pg_class c
	     	JOIN pg_attribute a ON a.attrelid = c.oid
	     	JOIN pg_type t ON t.oid = a.atttypid
	     	WHERE c.relname = 'relation'
	       AND a.attisdropped = false
	       AND t.typname = '_text'
	   	LOOP
	     	EXECUTE format($f$
	       	UPDATE "relation" r
	       	SET %1$I = (
	         	SELECT CASE WHEN COUNT(*) = 0 THEN NULL ELSE array_agg(e) END
	         	FROM unnest(r.%1$I) e
	         	WHERE NOT (e = ANY ($1))
	       	)
	       	WHERE r.%1$I IS NOT NULL
	         	AND EXISTS (SELECT 1 FROM unnest(r.%1$I) e WHERE e = ANY ($1));
		$f$, rec.colname) USING ids_text;
	
	     	EXECUTE format('UPDATE "relation" SET %1$I = NULL WHERE %1$I IS NOT NULL AND cardinality(%1$I) = 0;', rec.colname);
	   	END LOOP;
	
	 	END IF;
	
	 	DELETE FROM "field" WHERE slug = 'folder_id';
	 	RAISE NOTICE 'Deleted remaining rows from "field" where slug=folder_id.';
	
	 	DROP TABLE IF EXISTS folder_group;
	
		END $$;
	
		COMMIT;
`

	_, err := db.Exec(context.Background(), query)
	return err
}

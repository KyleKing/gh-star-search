-- Add VARCHAR columns for DuckDB FTS indexing (FTS cannot index JSON columns)
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS topics_text VARCHAR DEFAULT '';
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS contributors_text VARCHAR DEFAULT '';

-- Populate topics_text from existing topics_array JSON
UPDATE repositories SET topics_text = COALESCE(
    array_to_string(CAST(topics_array AS VARCHAR[]), ' '), ''
);

-- Populate contributors_text from existing contributors JSON
UPDATE repositories SET contributors_text = COALESCE(
    array_to_string(
        list_transform(CAST(contributors AS JSON[]),
            x -> json_extract_string(x, '$.Login')),
        ' '),
    ''
);

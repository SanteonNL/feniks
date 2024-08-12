    SELECT
    'Patient' as field_name,
    '' as parent_id,
    p.identificatienummer Id,
    -- CASE
    --     WHEN p.geslachtcode = 'M' THEN 'male'
    --     WHEN p.geslachtcode = 'F' THEN 'female'
    --     ELSE 'unknown'
    -- END as gender, 
    p.geboortedatum Birthdate
    FROM patient p
WHERE 1=1
AND p.identificatienummer = '123'
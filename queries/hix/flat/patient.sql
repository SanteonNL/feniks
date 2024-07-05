    SELECT
    p.identificatienummer id,
    CASE
        WHEN p.geslachtcode = 'M' THEN 'male'
        WHEN p.geslachtcode = 'F' THEN 'female'
        ELSE 'unknown'
    END as gender, 
    p.geboortedatum birthdate
    FROM patient p
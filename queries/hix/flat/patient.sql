    SELECT
    'Patient' as field_name,
    '' as parent_id,
    p.identificatienummer id,
    -- CASE
    --     WHEN p.geslachtcode = 'M' THEN 'male'
    --     WHEN p.geslachtcode = 'F' THEN 'female'
    --     ELSE 'unknown'
    -- END as gender, 
    p.geboortedatum birthdate
    FROM patient p
WHERE 1=1
 AND p.identificatienummer = '123';


SELECT
    'Patient.Name' as field_name,
    p.identificatienummer as parent_id,
    concat(p.identificatienummer,humanName.lastname) AS id,
    humanName.lastname as family,
    humanName.firstname AS name
FROM
    patient p
    JOIN names humanName ON humanName.identificatienummer = p.identificatienummer
WHERE 1=1
 AND p.identificatienummer = '123'   
GROUP BY
    p.identificatienummer, humanName.lastname,humanName.firstname;



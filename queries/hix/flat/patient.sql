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
 AND p.identificatienummer = '123';


WITH cte AS (
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
        p.identificatienummer, humanName.lastname,humanName.firstname
)
SELECT
        'Patient.Name' as field_name,
        cte.parent_id,
        concat(p.identificatienummer,humanName.lastname) AS id,
        humanName.lastname as family,
        humanName.firstname AS name
FROM cte





SELECT
    'Patient.Contact.Telecom' AS field_name,
    c.id AS parent_id,
    CONCAT(c.id, cp.system) AS id,
    cp.system,
    cp.value
FROM
    contacts c
    JOIN contact_points cp ON c.id = cp.contact_id
WHERE 1=1
 AND c.patient_id = '123'
GROUP BY
     cp.system, cp.value, c.id;


SELECT
    'Patient.Contact' AS field_name,
    p.identificatienummer AS parent_id,
    c.id AS id
FROM
    patient p
    JOIN contacts c ON c.patient_id = p.identificatienummer
WHERE 1=1
    AND p.identificatienummer = '123';

SELECT
    'Patient.Contact.Telecom' AS field_name,
    c.id AS parent_id,
    CONCAT(c.id, cp.system) AS id,
    cp.system,
    cp.value
FROM
    contacts c
    JOIN contact_points cp ON c.id = cp.contact_id
WHERE 1=1
 AND c.patient_id = '123'
GROUP BY
     cp.system, cp.value, c.id;
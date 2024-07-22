WITH patient_data AS (
    SELECT
        p.identificatienummer,
        humanName.lastname AS family,
        humanName.firstname AS name
    FROM
        patient p
        JOIN names humanName ON humanName.identificatienummer = p.identificatienummer
    WHERE
        p.identificatienummer = '123'
),
patient_contact AS (
    SELECT
        p.identificatienummer,
        c.id AS contact_id
    FROM
        patient p
        JOIN contacts c ON c.patient_id = p.identificatienummer
    WHERE
        p.identificatienummer = '123'
)
SELECT
    'Patient' AS field_name,
    '' AS parent_id,
    p.identificatienummer AS Id,
    p.geboortedatum AS Birthdate,
    NULL AS family,
    NULL AS name,
    NULL AS system,
    NULL AS value
FROM
    patient p
WHERE
    p.identificatienummer = '123'

UNION ALL

SELECT
    'Patient.Name' AS field_name,
    p.identificatienummer AS parent_id,
    CONCAT(p.identificatienummer, p.family) AS id,
    NULL AS Birthdate,
    p.family AS family,
    p.name AS name,
    NULL AS system,
    NULL AS value
FROM
    patient_data p

UNION ALL

SELECT
    'Patient.Contact' AS field_name,
    pc.identificatienummer AS parent_id,
    pc.contact_id AS id,
    NULL AS Birthdate,
    NULL AS family,
    NULL AS name,
    NULL AS system,
    NULL AS value
FROM
    patient_contact pc

UNION ALL

SELECT
    'Patient.Contact.Telecom' AS field_name,
    cp.contact_id AS parent_id,
    CONCAT(cp.contact_id, cp.system) AS id,
    NULL AS Birthdate,
    NULL AS family,
    NULL AS name,
    cp.system,
    cp.value
FROM
    patient_contact pc
    JOIN contact_points cp ON pc.contact_id = cp.contact_id
GROUP BY
    cp.contact_id, cp.system, cp.value;

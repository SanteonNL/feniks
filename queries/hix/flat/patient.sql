WITH names AS (
    SELECT
        'Patient' as fhir_path,
        p.identificatienummer as id,
        '' as parent_id,
        p.geboortedatum as birthDate,
        null as system,
        null as value,
        'male'as gender
    FROM
        patient p
    WHERE
        1 = 1
        AND p.identificatienummer = '123'
)
SELECT
    *
FROM
    names
UNION ALL
SELECT
    'Patient.identifier' as fhir_path,
    id as id,
    id as parent_id,
    null as birthDate,
    'https://santeon.nl' as system,
    id as value,
    null as gender
FROM
    names
UNION ALL
SELECT
    'Patient.identifier.type' as fhir_path,
    id,
    id as parent_id,
    null as birthDate,
    null as system,
    null as value,
    null as gender
FROM
    names
UNION ALL
SELECT
    'Patient.identifier' as fhir_path,
    '12345' as id,
    id as parent_id,
    null as birthDate,
    'https://santeon.nl' as system,
    '123456' as value,
    null as gender
FROM
    names
UNION ALL
SELECT
    'Patient.identifier.type' as fhir_path,
    '123435465' as id,
    '12345' as parent_id,
    null as birthDate,
    null as system,
    null as value,
    null as gender
FROM
    names;

-- The following part is commented out in the original query, so I'm keeping it commented
-- SELECT
--     'Patient.identifier' as fhir_path,
--     '123345' as id,
--     id as parent_id,
--     null as birthDate,
--     'https://santeon.nl' as system,
--     '12345' as value
-- FROM
--     names;
-- UNION ALL
-- SELECT
--     'Patient.identifier' as fhir_path,
--     id as id,
--     id as parent_id,
--     null as birthDate,
--     'https://santeon.nl' as system,
--     id as value
-- FROM
--     names
-- UNION ALL
-- SELECT
--     'Patient.identifier.type' as fhir_path,
--     id,
--     id as parent_id,
--     null as birthDate,
--     null as system,
--     null as value
-- FROM
--     names;

SELECT
    'Patient.identifier.type.coding' as fhir_path,
    '123435465'as parent_id,
    '1234' as id,
    'http://terminology.hl7.org/CodeSystem/v2-0203' as system,
    'MR' as code;
SELECT
    'Patient.identifier.type.coding' as fhir_path,
    '123'as parent_id,
    '12345' as id,
    'http://terminology.hl7.org/CodeSystem/v2-0203' as system,
    'AN' as code;

WITH names AS (
    SELECT
        'Patient.name' as fhir_path,
        p.identificatienummer as parent_id,
        concat(p.identificatienummer, humanName.lastname) AS id,
        humanName.lastname as family,
        JSON_ARRAY(humanName.firstname, 'Tommy', 'Jantine') AS given,
        '20120101' as "period.start",
        '20120102' as "period.end"
    FROM
        patient p
        JOIN names humanName ON humanName.identificatienummer = p.identificatienummer
    WHERE
        1 = 1
        AND p.identificatienummer = '123'
    GROUP BY
        p.identificatienummer,
        humanName.lastname,
        humanName.firstname
)
SELECT
    *
FROM
    names;



SELECT
    'Patient.contact.telecom' AS fhir_path,
    c.id AS parent_id,
    CONCAT(c.id, cp.system) AS id,
    cp.system,
    cp.value
FROM
    contacts c
    JOIN contact_points cp ON c.id = cp.contact_id
WHERE
    1 = 1
    AND c.patient_id = '123'
GROUP BY
    cp.system,
    cp.value,
    c.id;

SELECT
    'Patient.contact' AS fhir_path,
    p.identificatienummer AS parent_id,
    c.id AS id
FROM
    patient p
    JOIN contacts c ON c.patient_id = p.identificatienummer
WHERE
    1 = 1
    AND p.identificatienummer = '123';

SELECT
    'Patient.contact.telecom' AS fhir_path,
    c.id AS parent_id,
    CONCAT(c.id, cp.system) AS id,
    cp.system,
    cp.value
FROM
    contacts c
    JOIN contact_points cp ON c.id = cp.contact_id
WHERE
    1 = 1
    AND c.patient_id = '123'
GROUP BY
    cp.system,
    cp.value,
    c.id;
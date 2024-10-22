SELECT
    identificatienummer AS "id",
    'Patient' AS fhir_path,
    '' AS parent_id,
    '' AS resource_id,
    'generated' AS "text.status",
    '<div xmlns="http://www.w3.org/1999/xhtml">Patient information</div>' AS "text.div",
    'true' AS "active",
    'official' AS "name[0].use",
    --'Tommy' AS "name[0].given",
    'Hetterscheid' AS "name[0].family",
    '1992-01-01' AS "birthDate",
   -- 'male' AS "gender",
   -- '2e Daalsedijk 106' AS "address[0].line",
    '3565AA' AS "address[0].postalCode",
    'Utrecht' AS "address[0].city",
    'NL' AS "address[0].country",
    'home' AS "telecom[0].use",
    'phone' AS "telecom[0].system",
    '0650989181' AS "telecom[0].value",
    --'official' AS "identifier[0].use",
    'http://fhir.nl/fhir/NamingSystem/bsn' AS "identifier[0].system",
    '22221s' AS "identifier[0].value"
FROM
    patient
-- WHERE identificatienummer = :Patient.id
LIMIT 1;